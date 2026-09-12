package main

import (
	"io"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/renderer"
	goldmarkhtml "github.com/yuin/goldmark/v2/renderer/html"
)

// rawHTMLRendering preserves CommonMark raw HTML after the CLI has established
// that the deck is trusted. It overrides only raw HTML nodes, leaving
// Goldmark's dangerous-URL filtering in place for Markdown links and images.
var rawHTMLRendering goldmarkhtml.Extension = &rawHTMLRenderingExtension{}

type rawHTMLRenderingExtension struct{}

func (e *rawHTMLRenderingExtension) RendererOptions(_ *goldmarkhtml.Config) []goldmarkhtml.Option {
	renderer := &rawHTMLRenderer{}
	return []goldmarkhtml.Option{goldmarkhtml.WithNodeRenderers(map[ast.NodeKind]goldmarkhtml.NodeRenderer{
		ast.KindHTMLBlock: goldmarkhtml.NodeRendererFunc(renderer.renderHTMLBlock),
		ast.KindRawHTML:   goldmarkhtml.NodeRendererFunc(renderer.renderRawHTML),
	})}
}

type rawHTMLRenderer struct{}

func (r *rawHTMLRenderer) renderHTMLBlock(writer io.Writer, source []byte, node ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
	block := node.(*ast.HTMLBlock)
	if entering {
		_, _ = block.Value.WriteTo(goldmarkhtml.ContextHTMLWriter(rc), source)
	}
	return ast.WalkContinue, nil
}

func (r *rawHTMLRenderer) renderRawHTML(writer io.Writer, source []byte, node ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	raw := node.(*ast.RawHTML)
	_, _ = raw.Value.WriteTo(goldmarkhtml.ContextHTMLWriter(rc), source)
	return ast.WalkSkipChildren, nil
}
