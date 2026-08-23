package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
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

func TestRenderSlidesKeepsMermaidSourceEscaped(t *testing.T) {
	source := []byte("```mermaid\nflowchart LR\nA[\"<script>alert(1)</script>\"] --> B\n```\n")

	slides, err := renderSlides(source, "")
	if err != nil {
		t.Fatalf("renderSlides() error: %v", err)
	}
	html := string(slides[0])
	for _, expected := range []string{`class="language-mermaid"`, `&lt;script&gt;alert(1)&lt;/script&gt;`, `--&gt; B`} {
		if !strings.Contains(html, expected) {
			t.Errorf("rendered Mermaid source does not contain %q:\n%s", expected, html)
		}
	}
	if strings.Contains(html, "<script>") {
		t.Errorf("rendered Mermaid source contains executable script markup:\n%s", html)
	}
}

func TestRenderSlidesRejectsEmptyDeck(t *testing.T) {
	if _, err := renderSlides([]byte(" \n---\n "), ""); err == nil {
		t.Fatal("renderSlides() accepted an empty deck")
	}
}

func TestExternalMediaReferences(t *testing.T) {
	deckDirectory := filepath.Join(t.TempDir(), "deck")
	source := []byte(`![Local](image.png)
![Outside](../image.png)
![Absolute](/private/image.png)
![Remote](https://example.com/image.png)
![Duplicate](https://example.com/image.png)
![Embedded](data:image/png;base64,AAAA)`)
	if got, want := externalMediaReferences(source, deckDirectory), []string{"../image.png", "/private/image.png", "https://example.com/image.png"}; !slices.Equal(got, want) {
		t.Fatalf("externalMediaReferences() = %#v, want %#v", got, want)
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

func TestRenderSlidesEmbedsLocalVideoUsingImageSyntax(t *testing.T) {
	directory := t.TempDir()
	videoPath := filepath.Join(directory, "clip.mp4")
	if err := os.WriteFile(videoPath, []byte("not a real video"), 0o600); err != nil {
		t.Fatal(err)
	}

	slides, err := renderSlides([]byte("![Local clip](clip.mp4 \"A local video\")"), directory)
	if err != nil {
		t.Fatalf("renderSlides() error: %v", err)
	}
	html := string(slides[0])
	for _, expected := range []string{`<video controls preload="metadata" src="data:video/mp4;base64,`, `aria-label="Local clip"`, `title="A local video"`} {
		if !strings.Contains(html, expected) {
			t.Errorf("rendered local video does not contain %q:\n%s", expected, html)
		}
	}
}

func TestRenderSlidesKeepsRemoteVideoRemote(t *testing.T) {
	slides, err := renderSlides([]byte("![Remote clip](https://example.com/clip.webm)"), t.TempDir())
	if err != nil {
		t.Fatalf("renderSlides() error: %v", err)
	}
	html := string(slides[0])
	if !strings.Contains(html, `<video controls preload="metadata" src="https://example.com/clip.webm" aria-label="Remote clip"></video>`) {
		t.Fatalf("rendered remote video = %s", html)
	}
}

func TestRenderSlidesEmbedsAbsoluteLocalMediaPaths(t *testing.T) {
	mediaDirectory := t.TempDir()
	imagePath := filepath.Join(mediaDirectory, "absolute.png")
	videoPath := filepath.Join(mediaDirectory, "absolute.mp4")
	if err := os.WriteFile(imagePath, []byte("\x89PNG\r\n\x1a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(videoPath, []byte("not a real video"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		source string
		want   string
	}{
		{source: "![Absolute image](" + imagePath + ")", want: `src="data:image/png;base64,`},
		{source: "![Absolute video](file://" + videoPath + ")", want: `<video controls preload="metadata" src="data:video/mp4;base64,`},
	} {
		slides, err := renderSlides([]byte(test.source), t.TempDir())
		if err != nil {
			t.Fatalf("renderSlides(%q) error: %v", test.source, err)
		}
		if html := string(slides[0]); !strings.Contains(html, test.want) {
			t.Fatalf("rendered absolute media = %s", html)
		}
	}
}

func TestRenderSlidesRejectsUntrustedDataVideo(t *testing.T) {
	slides, err := renderSlides([]byte("![Unsafe](data:video/mp4;base64,AAAA)"), t.TempDir())
	if err != nil {
		t.Fatalf("renderSlides() error: %v", err)
	}
	if html := string(slides[0]); strings.Contains(html, "data:video/") {
		t.Fatalf("rendered untrusted data video: %s", html)
	}
}

func TestRenderSlidesSkipsMissingRelativeImage(t *testing.T) {
	var warnings bytes.Buffer
	slides, err := renderSlidesWithWarnings([]byte("# Available\n\n![Missing](missing.png)\n\nStill rendered."), t.TempDir(), &warnings)
	if err != nil {
		t.Fatalf("renderSlides() error: %v", err)
	}
	html := string(slides[0])
	if strings.Contains(html, "missing.png") {
		t.Fatalf("rendered HTML contains missing image reference: %s", html)
	}
	if !strings.Contains(html, "Still rendered.") {
		t.Fatalf("rendered HTML omitted remaining slide content: %s", html)
	}
	if warning := warnings.String(); !strings.Contains(warning, `md-present: warning: skip local media "missing.png"`) {
		t.Fatalf("missing media warning = %q", warning)
	}
}
