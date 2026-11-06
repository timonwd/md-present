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
	markdownSlides := parseSlides(source)
	slides := make([]string, len(markdownSlides))
	for index, slide := range markdownSlides {
		slides[index] = slide.source
	}
	return slides
}

type markdownSlide struct {
	source string
	hidden bool
}

type presentationSlide struct {
	HTML   template.HTML
	Hidden bool
}

type markdownSlideSegment struct {
	sourceRange
	hidden bool
}

func parseSlides(source string) []markdownSlide {
	segments := presentationSlideSegments(source)
	slides := make([]markdownSlide, len(segments))
	for index, segment := range segments {
		slides[index] = markdownSlide{source: source[segment.start:segment.stop], hidden: segment.hidden}
	}
	return slides
}

func slideSegments(source string) []sourceRange {
	segments := presentationSlideSegments(source)
	ranges := make([]sourceRange, len(segments))
	for index, segment := range segments {
		ranges[index] = segment.sourceRange
	}
	return ranges
}

func presentationSlideSegments(source string) []markdownSlideSegment {
	fencedRanges := fencedCodeRanges([]byte(source))
	start := documentFrontMatterEnd(source)
	lines := strings.SplitAfter(source[start:], "\n")
	lineOffsets := make([]int, len(lines))
	offset := start
	for index, line := range lines {
		lineOffsets[index] = offset
		offset += len(line)
	}

	var segments []markdownSlideSegment
	currentStart := start
	currentHidden := false
	appendSegment := func(stop int) {
		body := source[currentStart:stop]
		trimmedLeft := strings.TrimLeftFunc(body, unicode.IsSpace)
		trimmed := strings.TrimRightFunc(trimmedLeft, unicode.IsSpace)
		if trimmed != "" {
			contentStart := currentStart + len(body) - len(trimmedLeft)
			segments = append(segments, markdownSlideSegment{
				sourceRange: sourceRange{start: contentStart, stop: contentStart + len(trimmed)},
				hidden:      currentHidden,
			})
		}
	}

	for index := 0; index < len(lines); index++ {
		line := lines[index]
		lineStart := lineOffsets[index]
		lineEnd := lineStart + len(line)
		if strings.TrimSpace(line) == "---" && !overlapsRange(lineStart, lineEnd, fencedRanges) {
			if hidden, end, ok := slideFrontmatter(lines, lineOffsets, index, fencedRanges); ok {
				appendSegment(lineStart)
				currentStart = lineOffsets[end] + len(lines[end])
				currentHidden = hidden
				index = end
				continue
			}
			appendSegment(lineStart)
			currentStart = lineEnd
			currentHidden = false
		}
	}
	appendSegment(len(source))
	return segments
}

func documentFrontMatterEnd(source string) int {
	lines := strings.SplitAfter(source, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return 0
	}
	offset := len(lines[0])
	themeReferenceFound := false
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if themeReferenceFound {
				return offset + len(line)
			}
			return 0
		}
		if trimmed == "" {
			offset += len(line)
			continue
		}
		field, value, found := strings.Cut(trimmed, ":")
		if !found || strings.TrimSpace(field) != "theme" || themeReferenceFound || strings.Trim(strings.TrimSpace(value), `"'`) == "" {
			return 0
		}
		themeReferenceFound = true
		offset += len(line)
	}
	return 0
}

func isHiddenSlideFrontMatter(frontmatter string) bool {
	foundHidden := false
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		field, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(field) != "hidden" || foundHidden {
			return false
		}
		switch strings.TrimSpace(value) {
		case "true", "false":
			foundHidden = true
		default:
			return false
		}
	}
	return foundHidden
}

func slideFrontmatter(lines []string, lineOffsets []int, start int, fencedRanges []sourceRange) (hidden bool, end int, ok bool) {
	foundHidden := false
	for index := start + 1; index < len(lines); index++ {
		lineStart := lineOffsets[index]
		lineEnd := lineStart + len(lines[index])
		if overlapsRange(lineStart, lineEnd, fencedRanges) {
			return false, 0, false
		}
		if strings.TrimSpace(lines[index]) == "---" {
			return hidden, index, foundHidden
		}

		field, value, found := strings.Cut(strings.TrimSpace(lines[index]), ":")
		if !found || strings.TrimSpace(field) != "hidden" || foundHidden {
			return false, 0, false
		}
		switch strings.TrimSpace(value) {
		case "true":
			hidden = true
		case "false":
			hidden = false
		default:
			return false, 0, false
		}
		foundHidden = true
	}
	return false, 0, false
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
	presentationSlides, err := renderPresentationSlidesWithOptions(source, deckDirectory, warnings, options)
	if err != nil {
		return nil, err
	}
	rendered := make([]template.HTML, len(presentationSlides))
	for index, slide := range presentationSlides {
		rendered[index] = slide.HTML
	}
	return rendered, nil
}

func renderPresentationSlidesWithWarnings(source []byte, deckDirectory string, warnings io.Writer) ([]presentationSlide, error) {
	return renderPresentationSlidesWithOptions(source, deckDirectory, warnings, renderOptions{})
}

func renderPresentationSlidesWithOptions(source []byte, deckDirectory string, warnings io.Writer, options renderOptions) ([]presentationSlide, error) {
	var err error
	source, _, err = parseDeckTheme(source, deckDirectory)
	if err != nil {
		return nil, err
	}
	if rawHTMLPresent(source) && !options.allowRawHTML {
		return nil, fmt.Errorf("raw HTML requires --allow-raw-html")
	}
	warnUnterminatedFencedCodeBlocks(source, warnings)
	markdownSlides := parseSlides(string(source))
	if len(markdownSlides) == 0 {
		return nil, fmt.Errorf("the Markdown file contains no slide content")
	}

	renderer := newMarkdownRenderer(options)
	rendered := make([]presentationSlide, 0, len(markdownSlides))
	for _, slide := range markdownSlides {
		slideSource := []byte(slide.source)
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
		rendered = append(rendered, presentationSlide{HTML: template.HTML(output.String()), Hidden: slide.hidden}) //nolint:gosec
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
