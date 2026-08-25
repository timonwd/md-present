package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// This test-only extension proves that Goldmark can represent a safe layout
// container without enabling raw HTML. Production code should add validation
// and diagnostics before adopting the syntax.
var prototypeLayout goldmark.Extender = &prototypeLayoutExtension{}

var prototypeLayoutNodeKind = ast.NewNodeKind("PrototypeLayout")

type prototypeLayoutNode struct {
	ast.BaseBlock
	name        string
	fenceLength int
}

func (n *prototypeLayoutNode) Kind() ast.NodeKind {
	return prototypeLayoutNodeKind
}

func (n *prototypeLayoutNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Name": n.name}, nil)
}

type prototypeLayoutExtension struct{}

func (e *prototypeLayoutExtension) Extend(markdown goldmark.Markdown) {
	markdown.Parser().AddOptions(parser.WithBlockParsers(
		util.Prioritized(&prototypeLayoutParser{}, 650),
	))
	markdown.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&prototypeLayoutRenderer{}, 500),
	))
}

type prototypeLayoutParser struct{}

func (p *prototypeLayoutParser) Trigger() []byte {
	return []byte{':'}
}

func (p *prototypeLayoutParser) Open(parent ast.Node, reader text.Reader, context parser.Context) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	fenceLength, name, ok := prototypeLayoutFence(line, context.BlockOffset())
	if !ok {
		return nil, parser.NoChildren
	}

	switch name {
	case "columns":
		if fenceLength < 4 {
			return nil, parser.NoChildren
		}
	case "column":
		columns, isColumns := parent.(*prototypeLayoutNode)
		if fenceLength != 3 || !isColumns || columns.name != "columns" || fenceLength >= columns.fenceLength {
			return nil, parser.NoChildren
		}
	default:
		return nil, parser.NoChildren
	}

	reader.AdvanceToEOL()
	return &prototypeLayoutNode{name: name, fenceLength: fenceLength}, parser.HasChildren
}

func (p *prototypeLayoutParser) Continue(node ast.Node, reader text.Reader, context parser.Context) parser.State {
	layout := node.(*prototypeLayoutNode)
	line, _ := reader.PeekLine()
	fenceLength, name, ok := prototypeLayoutFence(line, context.BlockOffset())
	if ok && name == "" && fenceLength >= layout.fenceLength {
		reader.AdvanceToEOL()
		return parser.Close
	}
	return parser.Continue | parser.HasChildren
}

func (p *prototypeLayoutParser) Close(ast.Node, text.Reader, parser.Context) {}

func (p *prototypeLayoutParser) CanInterruptParagraph() bool {
	return true
}

func (p *prototypeLayoutParser) CanAcceptIndentedLine() bool {
	return false
}

func prototypeLayoutFence(line []byte, offset int) (int, string, bool) {
	if offset < 0 || offset >= len(line) || line[offset] != ':' {
		return 0, "", false
	}
	end := offset
	for end < len(line) && line[end] == ':' {
		end++
	}
	if end-offset < 3 {
		return 0, "", false
	}
	rest := strings.TrimSpace(string(line[end:]))
	if rest != "" && rest != "columns" && rest != "column" {
		return 0, "", false
	}
	return end - offset, rest, true
}

type prototypeLayoutRenderer struct{}

func (r *prototypeLayoutRenderer) RegisterFuncs(registerer renderer.NodeRendererFuncRegisterer) {
	registerer.Register(prototypeLayoutNodeKind, r.renderLayout)
}

func (r *prototypeLayoutRenderer) renderLayout(writer util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	layout := node.(*prototypeLayoutNode)
	if entering {
		_, _ = fmt.Fprintf(writer, `<div class="md-layout md-layout--%s">`+"\n", layout.name)
	} else {
		_, _ = writer.WriteString("</div>\n")
	}
	return ast.WalkContinue, nil
}

func TestPrototypeLayoutExtensionPreservesMarkdownSafetyAndMedia(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "pixel.png"), []byte("\x89PNG\r\n\x1a\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	source := []byte(`:::: columns
::: column
## Left

- **Nested Markdown**
- ![Local image](pixel.png)
:::
::: column
## Right

<script>alert("unsafe")</script>

![Remote image](https://example.com/remote.png)
:::
::::
`)
	markdown := newMarkdownRenderer()
	prototypeLayout.Extend(markdown)
	document := markdown.Parser().Parse(text.NewReader(source))
	if err := embedLocalMedia(document, directory, nil); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := markdown.Renderer().Render(&output, source, document); err != nil {
		t.Fatal(err)
	}
	html := output.String()

	for _, expected := range []string{
		`<div class="md-layout md-layout--columns">`,
		`<div class="md-layout md-layout--column">`,
		`<h2>Left</h2>`,
		`<strong>Nested Markdown</strong>`,
		`src="data:image/png;base64,`,
		`src="https://example.com/remote.png"`,
		`<!-- raw HTML omitted -->`,
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("prototype layout output does not contain %q:\n%s", expected, html)
		}
	}
	if got := strings.Count(html, `<div class="md-layout md-layout--column">`); got != 2 {
		t.Errorf("prototype layout rendered %d columns, want 2:\n%s", got, html)
	}
	for _, unsafe := range []string{"<script", ":::: columns", "::: column"} {
		if strings.Contains(strings.ToLower(html), strings.ToLower(unsafe)) {
			t.Errorf("prototype layout output contains %q:\n%s", unsafe, html)
		}
	}

	if got := externalMediaReferences(source, directory); len(got) != 1 || got[0] != "https://example.com/remote.png" {
		t.Fatalf("externalMediaReferences() = %#v, want the remote image inside the layout", got)
	}
}
