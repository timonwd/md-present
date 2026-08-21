package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

func newMarkdownRenderer() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.Linkify,
			extension.NewTable(extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute)),
			extension.Strikethrough,
			extension.TaskList,
			syntaxHighlighting,
		),
	)
}

func splitSlides(source string) []string {
	normalized := strings.ReplaceAll(source, "\r\n", "\n")
	fencedRanges := fencedCodeRanges([]byte(normalized))
	lines := strings.Split(normalized, "\n")
	var slides []string
	var current []string
	offset := 0

	appendSlide := func() {
		body := strings.TrimSpace(strings.Join(current, "\n"))
		if body != "" {
			slides = append(slides, body)
		}
		current = nil
	}

	for _, line := range lines {
		lineEnd := offset + len(line)
		if strings.TrimSpace(line) == "---" && !overlapsRange(offset, lineEnd, fencedRanges) {
			appendSlide()
		} else {
			current = append(current, line)
		}
		offset = lineEnd + 1
	}
	appendSlide()
	return slides
}

type sourceRange struct {
	start int
	stop  int
}

func fencedCodeRanges(source []byte) []sourceRange {
	markdown := newMarkdownRenderer()
	document := markdown.Parser().Parse(text.NewReader(source))
	var ranges []sourceRange
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node.Kind() != ast.KindFencedCodeBlock {
			return ast.WalkContinue, nil
		}
		lines := node.Lines()
		for i := 0; i < lines.Len(); i++ {
			segment := lines.At(i)
			ranges = append(ranges, sourceRange{start: segment.Start, stop: segment.Stop})
		}
		return ast.WalkContinue, nil
	})
	return ranges
}

func overlapsRange(start, stop int, ranges []sourceRange) bool {
	for _, candidate := range ranges {
		if start < candidate.stop && stop > candidate.start {
			return true
		}
	}
	return false
}

func renderSlides(source []byte, deckDirectory string) ([]template.HTML, error) {
	markdownSlides := splitSlides(string(source))
	if len(markdownSlides) == 0 {
		return nil, fmt.Errorf("the Markdown file contains no slide content")
	}

	renderer := newMarkdownRenderer()
	rendered := make([]template.HTML, 0, len(markdownSlides))
	for _, slide := range markdownSlides {
		slideSource := []byte(slide)
		document := renderer.Parser().Parse(text.NewReader(slideSource))
		if err := embedLocalImages(document, deckDirectory); err != nil {
			return nil, err
		}
		var output bytes.Buffer
		if err := renderer.Renderer().Render(&output, slideSource, document); err != nil {
			return nil, err
		}
		// Goldmark's safe renderer omits raw HTML and unsafe link schemes by default.
		rendered = append(rendered, template.HTML(output.String())) //nolint:gosec
	}
	return rendered, nil
}

func embedLocalImages(document ast.Node, deckDirectory string) error {
	return ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		image, ok := node.(*ast.Image)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		destination := string(image.Destination)
		parsed, err := url.Parse(destination)
		if err != nil || parsed.Scheme != "" || parsed.Host != "" || filepath.IsAbs(destination) {
			return ast.WalkContinue, nil
		}
		path, err := url.PathUnescape(parsed.Path)
		if err != nil {
			return ast.WalkStop, fmt.Errorf("decode image path %q: %w", destination, err)
		}
		if path == "" {
			return ast.WalkContinue, nil
		}
		data, err := os.ReadFile(filepath.Join(deckDirectory, filepath.FromSlash(path)))
		if err != nil {
			return ast.WalkStop, fmt.Errorf("read image %q: %w", destination, err)
		}
		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		image.Destination = []byte("data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data))
		return ast.WalkContinue, nil
	})
}
