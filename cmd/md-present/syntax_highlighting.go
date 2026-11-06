package main

import (
	"io"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/renderer"
	goldmarkhtml "github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/util"
)

// syntaxHighlighting adds server-side highlighting for fenced code blocks whose
// language is known to Chroma. It emits CSS classes instead of inline styles so
// the presentation's style-src 'self' content security policy remains intact.
var syntaxHighlighting goldmarkhtml.Extension = &syntaxHighlightingExtension{}

type syntaxHighlightingExtension struct{}

func (e *syntaxHighlightingExtension) RendererOptions(_ *goldmarkhtml.Config) []goldmarkhtml.Option {
	return []goldmarkhtml.Option{goldmarkhtml.WithNodeRenderers(map[ast.NodeKind]goldmarkhtml.NodeRenderer{
		ast.KindCodeBlock: goldmarkhtml.NodeRendererFunc(newSyntaxHighlightingRenderer().renderCodeBlock),
	})}
}

type syntaxHighlightingRenderer struct {
	formatter chroma.Formatter
}

func newSyntaxHighlightingRenderer() *syntaxHighlightingRenderer {
	return &syntaxHighlightingRenderer{
		formatter: chromahtml.New(
			chromahtml.WithClasses(true),
			chromahtml.PreventSurroundingPre(true),
		),
	}
}

func (r *syntaxHighlightingRenderer) renderCodeBlock(
	writer io.Writer,
	source []byte,
	node ast.Node,
	entering bool,
	_ renderer.Context,
) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	w := writer.(util.BufWriter)

	block := node.(*ast.CodeBlock)
	language, hasLanguage := block.Language(source)
	lexer := lexers.Get(language)
	if block.CodeBlockKind != ast.CodeBlockKindFenced || !hasLanguage || lexer == nil {
		renderPlainFencedCodeBlock(w, source, block, language)
		return ast.WalkSkipChildren, nil
	}

	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, string(block.Value.Bytes(source)))
	if err != nil {
		renderPlainFencedCodeBlock(w, source, block, language)
		return ast.WalkSkipChildren, nil
	}

	_, _ = w.WriteString(`<pre class="chroma"><code class="language-`)
	_, _ = w.Write(util.EscapeHTML([]byte(language)))
	_, _ = w.WriteString(`">`)
	if err := r.formatter.Format(w, styles.Fallback, iterator); err != nil {
		return ast.WalkStop, err
	}
	_, _ = w.WriteString("</code></pre>\n")
	return ast.WalkSkipChildren, nil
}

func renderPlainFencedCodeBlock(w util.BufWriter, source []byte, block *ast.CodeBlock, language string) {
	_, _ = w.WriteString("<pre><code")
	if language != "" {
		_, _ = w.WriteString(` class="language-`)
		_, _ = w.Write(util.EscapeHTML([]byte(language)))
		_ = w.WriteByte('"')
	}
	_ = w.WriteByte('>')
	_, _ = w.Write(util.EscapeHTML(block.Value.Bytes(source)))
	_, _ = w.WriteString("</code></pre>\n")
}
