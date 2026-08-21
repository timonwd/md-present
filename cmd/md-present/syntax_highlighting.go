package main

import (
	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// syntaxHighlighting adds server-side highlighting for fenced code blocks whose
// language is known to Chroma. It emits CSS classes instead of inline styles so
// the presentation's style-src 'self' content security policy remains intact.
var syntaxHighlighting goldmark.Extender = &syntaxHighlightingExtension{}

type syntaxHighlightingExtension struct{}

func (e *syntaxHighlightingExtension) Extend(markdown goldmark.Markdown) {
	markdown.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(newSyntaxHighlightingRenderer(), 500),
	))
}

type syntaxHighlightingRenderer struct {
	formatter chroma.Formatter
}

func newSyntaxHighlightingRenderer() renderer.NodeRenderer {
	return &syntaxHighlightingRenderer{
		formatter: chromahtml.New(
			chromahtml.WithClasses(true),
			chromahtml.PreventSurroundingPre(true),
		),
	}
}

func (r *syntaxHighlightingRenderer) RegisterFuncs(registerer renderer.NodeRendererFuncRegisterer) {
	registerer.Register(ast.KindFencedCodeBlock, r.renderFencedCodeBlock)
}

func (r *syntaxHighlightingRenderer) renderFencedCodeBlock(
	w util.BufWriter,
	source []byte,
	node ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	block := node.(*ast.FencedCodeBlock)
	language := block.Language(source)
	lexer := lexers.Get(string(language))
	if len(language) == 0 || lexer == nil {
		renderPlainFencedCodeBlock(w, source, block, language)
		return ast.WalkSkipChildren, nil
	}

	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, string(block.Lines().Value(source)))
	if err != nil {
		renderPlainFencedCodeBlock(w, source, block, language)
		return ast.WalkSkipChildren, nil
	}

	_, _ = w.WriteString(`<pre class="chroma"><code class="language-`)
	goldmarkhtml.DefaultWriter.Write(w, language)
	_, _ = w.WriteString(`">`)
	if err := r.formatter.Format(w, styles.Fallback, iterator); err != nil {
		return ast.WalkStop, err
	}
	_, _ = w.WriteString("</code></pre>\n")
	return ast.WalkSkipChildren, nil
}

func renderPlainFencedCodeBlock(w util.BufWriter, source []byte, block *ast.FencedCodeBlock, language []byte) {
	_, _ = w.WriteString("<pre><code")
	if len(language) > 0 {
		_, _ = w.WriteString(` class="language-`)
		goldmarkhtml.DefaultWriter.Write(w, language)
		_ = w.WriteByte('"')
	}
	_ = w.WriteByte('>')
	for i := 0; i < block.Lines().Len(); i++ {
		line := block.Lines().At(i)
		goldmarkhtml.DefaultWriter.RawWrite(w, line.Value(source))
	}
	_, _ = w.WriteString("</code></pre>\n")
}
