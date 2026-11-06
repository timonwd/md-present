package main

import (
	"html"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/renderer"
	goldmarkhtml "github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/util"
)

// videoRendering lets image-style Markdown embed browser-playable video while
// keeping Goldmark's safe handling for every other image destination.
var videoRendering goldmarkhtml.Extension = &videoRenderingExtension{}

type videoRenderingExtension struct{}

func (e *videoRenderingExtension) RendererOptions(_ *goldmarkhtml.Config) []goldmarkhtml.Option {
	return []goldmarkhtml.Option{goldmarkhtml.WithNodeRenderers(map[ast.NodeKind]goldmarkhtml.NodeRenderer{
		ast.KindImage: goldmarkhtml.NodeRendererFunc(newVideoRenderer().renderImage),
	})}
}

type videoRenderer struct{}

func newVideoRenderer() *videoRenderer { return &videoRenderer{} }

func (r *videoRenderer) renderImage(writer io.Writer, source []byte, node ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	w := writer.(util.BufWriter)

	image := node.(*ast.Image)
	destination := image.Destination.Value(source)
	if !isVideoDestination([]byte(destination)) {
		return renderStandardImage(w, source, image, destination, rc)
	}
	if goldmarkhtml.IsDangerousURL(destination) {
		if _, embedded := image.Attribute("md-present-embedded-video"); !embedded {
			return ast.WalkSkipChildren, nil
		}
	}

	_, _ = w.WriteString(`<video controls preload="metadata" src="`)
	_, _ = goldmarkhtml.ContextLinkURLWriter(rc).WriteString(destination)
	_ = w.WriteByte('"')
	if label := imageLabel(source, image); label != "" {
		_, _ = w.WriteString(` aria-label="`)
		_, _ = w.WriteString(html.EscapeString(label))
		_ = w.WriteByte('"')
	}
	if !image.Title.IsEmpty() {
		_, _ = w.WriteString(` title="`)
		_, _ = image.Title.WriteTo(goldmarkhtml.ContextTextWriter(rc), source)
		_ = w.WriteByte('"')
	}
	_, _ = w.WriteString(`></video>`)
	return ast.WalkSkipChildren, nil
}

func renderStandardImage(w util.BufWriter, source []byte, image *ast.Image, destination string, rc renderer.Context) (ast.WalkStatus, error) {
	_, _ = w.WriteString(`<img src="`)
	if !goldmarkhtml.IsDangerousURL(destination) {
		_, _ = goldmarkhtml.ContextLinkURLWriter(rc).WriteString(destination)
	}
	_, _ = w.WriteString(`" alt="`)
	_, _ = w.WriteString(html.EscapeString(imageLabel(source, image)))
	_ = w.WriteByte('"')
	if !image.Title.IsEmpty() {
		_, _ = w.WriteString(` title="`)
		_, _ = image.Title.WriteTo(goldmarkhtml.ContextTextWriter(rc), source)
		_ = w.WriteByte('"')
	}
	if image.Attributes() != nil {
		goldmarkhtml.RenderAttributes(w, source, image, goldmarkhtml.ImageAttributeFilter, rc)
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
			label.Write(value.Value.Bytes(source))
		}
		return ast.WalkContinue, nil
	})
	return label.String()
}
