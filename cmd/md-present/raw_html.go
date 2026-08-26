package main

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// rawHTMLRendering preserves CommonMark raw HTML after the CLI has established
// that the deck is trusted. It overrides only raw HTML nodes, leaving
// Goldmark's dangerous-URL filtering in place for Markdown links and images.
var rawHTMLRendering goldmark.Extender = &rawHTMLRenderingExtension{}

type rawHTMLRenderingExtension struct{}

func (e *rawHTMLRenderingExtension) Extend(markdown goldmark.Markdown) {
	markdown.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&rawHTMLRenderer{}, 500),
	))
}

type rawHTMLRenderer struct{}

func (r *rawHTMLRenderer) RegisterFuncs(registerer renderer.NodeRendererFuncRegisterer) {
	registerer.Register(ast.KindHTMLBlock, r.renderHTMLBlock)
	registerer.Register(ast.KindRawHTML, r.renderRawHTML)
}

func (r *rawHTMLRenderer) renderHTMLBlock(writer util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	block := node.(*ast.HTMLBlock)
	if entering {
		for i := 0; i < block.Lines().Len(); i++ {
			line := block.Lines().At(i)
			goldmarkhtml.DefaultWriter.SecureWrite(writer, line.Value(source))
		}
	} else if block.HasClosure() {
		goldmarkhtml.DefaultWriter.SecureWrite(writer, block.ClosureLine.Value(source))
	}
	return ast.WalkContinue, nil
}

func (r *rawHTMLRenderer) renderRawHTML(writer util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	raw := node.(*ast.RawHTML)
	for i := 0; i < raw.Segments.Len(); i++ {
		segment := raw.Segments.At(i)
		goldmarkhtml.DefaultWriter.SecureWrite(writer, segment.Value(source))
	}
	return ast.WalkSkipChildren, nil
}
