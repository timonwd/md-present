package main

import (
	"html"
	"net/url"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// videoRendering lets image-style Markdown embed browser-playable video while
// keeping Goldmark's safe handling for every other image destination.
var videoRendering goldmark.Extender = &videoRenderingExtension{}

type videoRenderingExtension struct{}

func (e *videoRenderingExtension) Extend(markdown goldmark.Markdown) {
	markdown.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(newVideoRenderer(), 500),
	))
}

type videoRenderer struct{}

func newVideoRenderer() renderer.NodeRenderer { return &videoRenderer{} }

func (r *videoRenderer) RegisterFuncs(registerer renderer.NodeRendererFuncRegisterer) {
	registerer.Register(ast.KindImage, r.renderImage)
}

func (r *videoRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	image := node.(*ast.Image)
	destination := util.URLEscape(image.Destination, true)
	if !isVideoDestination(image.Destination) {
		return renderStandardImage(w, source, image, destination)
	}
	if goldmarkhtml.IsDangerousURL(destination) {
		if _, embedded := image.AttributeString("md-present-embedded-video"); !embedded {
			return ast.WalkSkipChildren, nil
		}
	}

	_, _ = w.WriteString(`<video controls preload="metadata" src="`)
	_, _ = w.WriteString(html.EscapeString(string(destination)))
	_ = w.WriteByte('"')
	if label := imageLabel(source, image); label != "" {
		_, _ = w.WriteString(` aria-label="`)
		_, _ = w.WriteString(html.EscapeString(label))
		_ = w.WriteByte('"')
	}
	if image.Title != nil {
		_, _ = w.WriteString(` title="`)
		goldmarkhtml.DefaultWriter.Write(w, image.Title)
		_ = w.WriteByte('"')
	}
	_, _ = w.WriteString(`></video>`)
	return ast.WalkSkipChildren, nil
}

func renderStandardImage(w util.BufWriter, source []byte, image *ast.Image, destination []byte) (ast.WalkStatus, error) {
	_, _ = w.WriteString(`<img src="`)
	if !goldmarkhtml.IsDangerousURL(destination) {
		_, _ = w.Write(util.EscapeHTML(destination))
	}
	_, _ = w.WriteString(`" alt="`)
	_, _ = w.WriteString(html.EscapeString(imageLabel(source, image)))
	_ = w.WriteByte('"')
	if image.Title != nil {
		_, _ = w.WriteString(` title="`)
		goldmarkhtml.DefaultWriter.Write(w, image.Title)
		_ = w.WriteByte('"')
	}
	if image.Attributes() != nil {
		goldmarkhtml.RenderAttributes(w, image, goldmarkhtml.ImageAttributeFilter)
	}
	_ = w.WriteByte('>')
	return ast.WalkSkipChildren, nil
}

func isVideoDestination(destination []byte) bool {
	value := string(destination)
	if strings.HasPrefix(strings.ToLower(value), "data:video/") {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".mp4", ".m4v", ".mov", ".ogv", ".ogg", ".webm":
		return true
	default:
		return false
	}
}

func imageLabel(source []byte, image *ast.Image) string {
	var label strings.Builder
	_ = ast.Walk(image, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch value := node.(type) {
		case *ast.Text:
			label.Write(value.Segment.Value(source))
		case *ast.String:
			label.Write(value.Value)
		}
		return ast.WalkContinue, nil
	})
	return label.String()
}
