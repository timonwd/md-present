package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	goldmarkhtml "github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/text"
)

type renderOptions struct {
	allowRawHTML bool
	markdownPath string
	themeCSS     template.CSS
	themePath    string
	themeFooter  string
}

type markdownRenderer struct {
	parser   parser.Parser
	renderer goldmarkhtml.Renderer
}

func newMarkdownRenderer(options renderOptions) markdownRenderer {
	parserExtensions := []parser.Extension{
		extension.NewLinkifyParser(),
		extension.NewTableParser(),
		extension.NewStrikethroughParser(),
		extension.NewTaskListItemParser(),
	}
	rendererExtensions := []goldmarkhtml.Extension{
		extension.NewTableHTMLRenderer(extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute)),
		extension.NewStrikethroughHTMLRenderer(),
		extension.NewTaskListItemHTMLRenderer(),
		syntaxHighlighting,
		videoRendering,
	}
	if options.allowRawHTML {
		rendererExtensions = append(rendererExtensions, rawHTMLRendering)
	}
	return markdownRenderer{
		parser:   parser.New(parser.WithExtensions(parserExtensions...)),
		renderer: goldmarkhtml.New(goldmarkhtml.WithExtensions(rendererExtensions...)),
	}
}

func splitSlides(source string) []string {
	segments := slideSegments(source)
	slides := make([]string, len(segments))
	for index, segment := range segments {
		slides[index] = source[segment.start:segment.stop]
	}
	return slides
}

func slideSegments(source string) []sourceRange {
	fencedRanges := fencedCodeRanges([]byte(source))
	start := 0
	if strings.HasPrefix(source, "---\n") {
		if end := strings.Index(source[4:], "\n---\n"); end >= 0 {
			start = 4 + end + len("\n---\n")
		}
	}
	lines := strings.SplitAfter(source[start:], "\n")
	var segments []sourceRange
	offset := start
	appendSegment := func(stop int) {
		body := source[start:stop]
		trimmedLeft := strings.TrimLeftFunc(body, unicode.IsSpace)
		trimmed := strings.TrimRightFunc(trimmedLeft, unicode.IsSpace)
		if trimmed != "" {
			contentStart := start + len(body) - len(trimmedLeft)
			segments = append(segments, sourceRange{start: contentStart, stop: contentStart + len(trimmed)})
		}
	}

	for _, line := range lines {
		lineEnd := offset + len(line)
		if strings.TrimSpace(line) == "---" && !overlapsRange(offset, lineEnd, fencedRanges) {
			appendSegment(offset)
			start = lineEnd
		}
		offset = lineEnd
	}
	appendSegment(len(source))
	return segments
}

type sourceRange struct {
	start int
	stop  int
}

func fencedCodeRanges(source []byte) []sourceRange {
	markdown := newMarkdownRenderer(renderOptions{})
	document := markdown.parser.Parse(source)
	var ranges []sourceRange
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		block, ok := node.(*ast.CodeBlock)
		if !entering || !ok || block.CodeBlockKind != ast.CodeBlockKindFenced {
			return ast.WalkContinue, nil
		}
		for _, segment := range block.Value.Segments() {
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
	return renderSlidesWithOptions(source, deckDirectory, nil, renderOptions{})
}

func renderSlidesWithWarnings(source []byte, deckDirectory string, warnings io.Writer) ([]template.HTML, error) {
	return renderSlidesWithOptions(source, deckDirectory, warnings, renderOptions{})
}

func renderSlidesWithOptions(source []byte, deckDirectory string, warnings io.Writer, options renderOptions) ([]template.HTML, error) {
	var err error
	source, _, err = parseDeckTheme(source, deckDirectory)
	if err != nil {
		return nil, err
	}
	if rawHTMLPresent(source) && !options.allowRawHTML {
		return nil, fmt.Errorf("raw HTML requires --allow-raw-html")
	}
	warnUnterminatedFencedCodeBlocks(source, warnings)
	markdownSlides := splitSlides(string(source))
	if len(markdownSlides) == 0 {
		return nil, fmt.Errorf("the Markdown file contains no slide content")
	}

	renderer := newMarkdownRenderer(options)
	rendered := make([]template.HTML, 0, len(markdownSlides))
	for _, slide := range markdownSlides {
		slideSource := []byte(slide)
		document := renderer.parser.Parse(slideSource)
		if err := embedLocalMedia(document, slideSource, deckDirectory, warnings); err != nil {
			return nil, err
		}
		var output bytes.Buffer
		if err := renderer.renderer.Render(&output, slideSource, document); err != nil {
			return nil, err
		}
		// Raw HTML reaches this point only after the CLI trust gate. Markdown
		// links retain Goldmark's dangerous-URL filtering.
		rendered = append(rendered, template.HTML(output.String())) //nolint:gosec
	}
	return rendered, nil
}

func warnUnterminatedFencedCodeBlocks(source []byte, warnings io.Writer) {
	if warnings == nil {
		return
	}

	markdown := newMarkdownRenderer(renderOptions{})
	document := markdown.parser.Parse(source)
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		block, ok := node.(*ast.CodeBlock)
		if !entering || !ok || block.CodeBlockKind != ast.CodeBlockKindFenced {
			return ast.WalkContinue, nil
		}
		lines := block.Value.Segments()
		if len(lines) == 0 || lines[len(lines)-1].Stop != len(source) {
			return ast.WalkContinue, nil
		}
		fmt.Fprintln(warnings, "Warning: unterminated fenced code block continues to the end of the document.")
		return ast.WalkContinue, nil
	})
}

func externalMediaReferences(source []byte, deckDirectory string) []string {
	document := newMarkdownRenderer(renderOptions{}).parser.Parse(source)
	seen := make(map[string]struct{})
	var references []string
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		image, ok := node.(*ast.Image)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		destination := image.Destination.Value(source)
		if !isExternalMediaDestination(destination, deckDirectory) {
			return ast.WalkContinue, nil
		}
		if _, exists := seen[destination]; exists {
			return ast.WalkContinue, nil
		}
		seen[destination] = struct{}{}
		references = append(references, destination)
		return ast.WalkContinue, nil
	})
	return references
}

func localMediaPaths(source []byte, deckDirectory string) []string {
	document := newMarkdownRenderer(renderOptions{}).parser.Parse(source)
	seen := make(map[string]struct{})
	var paths []string
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		image, ok := node.(*ast.Image)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		path, ok := localMediaPath(image.Destination.Value(source), deckDirectory)
		if !ok {
			return ast.WalkContinue, nil
		}
		if _, exists := seen[path]; exists {
			return ast.WalkContinue, nil
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
		return ast.WalkContinue, nil
	})
	return paths
}

func rawHTMLPresent(source []byte) bool {
	document := newMarkdownRenderer(renderOptions{}).parser.Parse(source)
	found := false
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && (node.Kind() == ast.KindHTMLBlock || node.Kind() == ast.KindRawHTML) {
			found = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return found
}

func localMediaPath(destination, deckDirectory string) (string, bool) {
	parsed, err := url.Parse(destination)
	if err != nil || (parsed.Scheme == "file" && parsed.Host != "" && parsed.Host != "localhost") || (parsed.Scheme != "" && parsed.Scheme != "file") || (parsed.Host != "" && parsed.Scheme != "file") {
		return "", false
	}
	path, err := url.PathUnescape(parsed.Path)
	if err != nil || path == "" {
		return "", false
	}
	path = filepath.FromSlash(path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(deckDirectory, path)
	}
	return path, true
}

func isExternalMediaDestination(destination, deckDirectory string) bool {
	parsed, err := url.Parse(destination)
	if err != nil || parsed.Scheme == "data" {
		return false
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" || parsed.Scheme == "file" || parsed.Host != "" {
		return true
	}
	if parsed.Scheme != "" {
		return false
	}
	path, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return false
	}
	if filepath.IsAbs(path) {
		return true
	}
	return pathIsOutsideDirectory(filepath.Join(deckDirectory, filepath.FromSlash(path)), deckDirectory)
}

func pathIsOutsideDirectory(path, directory string) bool {
	resolvedPath, pathErr := filepath.EvalSymlinks(path)
	resolvedDirectory, directoryErr := filepath.EvalSymlinks(directory)
	if pathErr == nil && directoryErr == nil {
		path, directory = resolvedPath, resolvedDirectory
	}
	relative, err := filepath.Rel(directory, path)
	return err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func embedLocalMedia(document ast.Node, source []byte, deckDirectory string, warnings io.Writer) error {
	return ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		image, ok := node.(*ast.Image)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		destination := image.Destination.Value(source)
		parsed, err := url.Parse(destination)
		if err != nil {
			return ast.WalkContinue, nil
		}
		if parsed.Scheme == "file" {
			if parsed.Host != "" && parsed.Host != "localhost" {
				return ast.WalkStop, fmt.Errorf("read image %q: file URL host is not supported", destination)
			}
		} else if parsed.Scheme != "" || parsed.Host != "" {
			return ast.WalkContinue, nil
		}
		path, err := url.PathUnescape(parsed.Path)
		if err != nil {
			return ast.WalkStop, fmt.Errorf("decode image path %q: %w", destination, err)
		}
		if path == "" {
			return ast.WalkContinue, nil
		}
		path = filepath.FromSlash(path)
		if !filepath.IsAbs(path) {
			path = filepath.Join(deckDirectory, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			// A local media reference can become stale while a deck is being
			// edited. Leaving it out keeps the rest of the presentation usable;
			// the server deliberately does not expose deck files for the browser
			// to retry this request itself.
			parent := image.Parent()
			if parent != nil {
				parent.RemoveChild(image)
			}
			if warnings != nil {
				fmt.Fprintf(warnings, "Warning: skipped local media %q: %v\n", destination, err)
			}
			return ast.WalkContinue, nil
		}
		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		image.Destination = text.NewSingleLineValueFromString("data:"+contentType+";base64,"+base64.StdEncoding.EncodeToString(data), text.IdentityDecoder)
		if strings.HasPrefix(contentType, "video/") {
			image.SetAttribute("md-present-embedded-video", text.NewMultiLineValue("true", text.IdentityDecoder))
		}
		return ast.WalkContinue, nil
	})
}
