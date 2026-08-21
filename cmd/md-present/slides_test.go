package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitSlides(t *testing.T) {
	source := `
---
# One

   ---

## Two

` + "```text" + `
---
` + "```" + `

- Nested fence

  ` + "```text" + `
  ---
  ` + "```" + `

---

~~~
---
~~~~
---
`

	slides := splitSlides(source)
	if len(slides) != 3 {
		t.Fatalf("splitSlides() returned %d slides, want 3: %#v", len(slides), slides)
	}
	if slides[0] != "# One" {
		t.Fatalf("first slide = %q", slides[0])
	}
	if !strings.Contains(slides[1], "```text\n---\n```") {
		t.Fatalf("backtick fence was split: %q", slides[1])
	}
	if !strings.Contains(slides[1], "```text\n  ---\n  ```") {
		t.Fatalf("nested fence was split: %q", slides[1])
	}
	if !strings.Contains(slides[2], "~~~\n---\n~~~~") {
		t.Fatalf("tilde fence was split: %q", slides[2])
	}
}

func TestSplitSlidesIgnoresOnlyEmptyEdges(t *testing.T) {
	slides := splitSlides("\n --- \n# Only\n---\n\n")
	if len(slides) != 1 || slides[0] != "# Only" {
		t.Fatalf("splitSlides() = %#v, want one non-empty slide", slides)
	}
}

func TestRenderSlidesMarkdownAndSafety(t *testing.T) {
	source := []byte(`# Title

A [safe link](https://example.com), [unsafe link](javascript:alert(1)), and ![image](data:image/png;base64,AAAA).

<script>alert("bad")</script>

> Quote

- Item

` + "```go" + `
fmt.Println("hello")
` + "```" + "\n")

	slides, err := renderSlides(source, "")
	if err != nil {
		t.Fatalf("renderSlides() error: %v", err)
	}
	if len(slides) != 1 {
		t.Fatalf("renderSlides() returned %d slides", len(slides))
	}
	html := string(slides[0])
	for _, expected := range []string{"<h1>Title</h1>", "<blockquote>", "<ul>", "<pre class=\"chroma\"><code class=\"language-go\">", "<img"} {
		if !strings.Contains(html, expected) {
			t.Errorf("rendered HTML does not contain %q:\n%s", expected, html)
		}
	}
	for _, unsafe := range []string{"<script", "javascript:alert"} {
		if strings.Contains(strings.ToLower(html), unsafe) {
			t.Errorf("rendered HTML contains unsafe value %q:\n%s", unsafe, html)
		}
	}
}

func TestRenderSlidesGFM(t *testing.T) {
	source := []byte(`| Feature | Ready |
| --- | ---: |
| Tables | yes |

~~Completed~~ and https://example.com/docs

- [x] Render GFM
- [ ] Add Mermaid
`)

	slides, err := renderSlides(source, "")
	if err != nil {
		t.Fatalf("renderSlides() error: %v", err)
	}
	html := string(slides[0])
	for _, expected := range []string{
		"<table>",
		"<th>Feature</th>",
		`<th align="right">Ready</th>`,
		"<del>Completed</del>",
		`<a href="https://example.com/docs">https://example.com/docs</a>`,
		`<input checked="" disabled="" type="checkbox">`,
		`<input disabled="" type="checkbox">`,
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("rendered GFM HTML does not contain %q:\n%s", expected, html)
		}
	}
	if strings.Contains(html, `style="text-align:right"`) {
		t.Errorf("rendered GFM table uses a CSP-blocked inline style:\n%s", html)
	}
}

func TestRenderSlidesRejectsEmptyDeck(t *testing.T) {
	if _, err := renderSlides([]byte(" \n---\n "), ""); err == nil {
		t.Fatal("renderSlides() accepted an empty deck")
	}
}

func TestRenderSlidesEmbedsRelativeImages(t *testing.T) {
	directory := t.TempDir()
	imagePath := filepath.Join(directory, "pixel.png")
	if err := os.WriteFile(imagePath, []byte("\x89PNG\r\n\x1a\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	slides, err := renderSlides([]byte("![Pixel](pixel.png)"), directory)
	if err != nil {
		t.Fatalf("renderSlides() error: %v", err)
	}
	if html := string(slides[0]); !strings.Contains(html, `src="data:image/png;base64,`) {
		t.Fatalf("relative image was not embedded: %s", html)
	}
}

func TestRenderSlidesReportsMissingRelativeImage(t *testing.T) {
	_, err := renderSlides([]byte("![Missing](missing.png)"), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), `read image "missing.png"`) {
		t.Fatalf("missing image error = %v", err)
	}
}
