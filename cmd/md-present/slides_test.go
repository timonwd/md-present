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
	for _, unsafe := range []string{"javascript:alert"} {
		if strings.Contains(strings.ToLower(html), unsafe) {
			t.Errorf("rendered HTML contains unsafe value %q:\n%s", unsafe, html)
		}
	}
}

func TestRawHTMLPresent(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   bool
	}{
		{name: "block", source: "<div>Layout</div>\n", want: true},
		{name: "inline", source: "Text with <mark>HTML</mark>.\n", want: true},
		{name: "comment", source: "<!-- presenter hint -->\n", want: true},
		{name: "code", source: "```html\n<div>Example</div>\n```\n", want: false},
		{name: "escaped", source: "\\<div>Literal\\</div>\n", want: false},
		{name: "markdown", source: "# Plain Markdown\n", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := rawHTMLPresent([]byte(test.source)); got != test.want {
				t.Fatalf("rawHTMLPresent() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRenderSlidesRequiresTrustedRawHTML(t *testing.T) {
	_, err := renderSlides([]byte("<div>Layout</div>\n"), "")
	if err == nil || !strings.Contains(err.Error(), "--allow-raw-html") {
		t.Fatalf("renderSlides() error = %v, want raw HTML trust error", err)
	}
}

func TestRenderSlidesTrustedRawHTMLPreservesCommonMarkAndURLSafety(t *testing.T) {
	source := []byte(`<div class="columns">

<div class="column">

## Left

- **Nested Markdown**

</div>

<div class="column">

Right with <mark data-note="trusted">inline HTML</mark>.

</div>

</div>

[Unsafe Markdown link](javascript:alert(1))
`)

	slides, err := renderSlidesWithOptions(source, "", nil, renderOptions{allowRawHTML: true})
	if err != nil {
		t.Fatalf("renderSlidesWithOptions() error: %v", err)
	}
	html := string(slides[0])
	for _, expected := range []string{
		`<div class="columns">`,
		`<div class="column">`,
		`<h2>Left</h2>`,
		`<strong>Nested Markdown</strong>`,
		`<mark data-note="trusted">inline HTML</mark>`,
	} {
		if !strings.Contains(html, expected) {
			t.Errorf("trusted raw HTML output does not contain %q:\n%s", expected, html)
		}
	}
	if got := strings.Count(html, `<div class="column">`); got != 2 {
		t.Errorf("trusted raw HTML rendered %d columns, want 2:\n%s", got, html)
	}
	if strings.Contains(strings.ToLower(html), "javascript:alert") {
		t.Errorf("trusted raw HTML enabled an unsafe Markdown link:\n%s", html)
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

func TestMermaidTypesFixtureHasBaselineAndErrorCases(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "mermaid-types.md"))
	if err != nil {
		t.Fatal(err)
	}

	slides, err := renderSlides(source, "")
	if err != nil {
		t.Fatalf("renderSlides() error: %v", err)
	}
	if len(slides) != 11 {
		t.Fatalf("renderSlides() returned %d slides, want 11", len(slides))
	}

	for _, expected := range []string{
		"flowchart LR",
		"A([Start]) --> B[Step 1]",
		"B --> C[Step 2]",
		"C --> D([End])",
		"sequenceDiagram",
		"classDiagram",
		"stateDiagram-v2",
		"erDiagram",
		"timeline",
		"mindmap",
		"architecture-beta",
		"this is not Mermaid",
		"%%{init:",
	} {
		if !strings.Contains(string(source), expected) {
			t.Errorf("Mermaid fixture does not contain %q", expected)
		}
	}

	mermaidSlides := 0
	for _, slide := range slides {
		if strings.Contains(string(slide), `class="language-mermaid"`) {
			mermaidSlides++
		}
	}
	if mermaidSlides != 10 {
		t.Errorf("fixture has %d Mermaid slides, want 10", mermaidSlides)
	}
}

func TestLayoutsFixtureUsesTrustedCommonMarkHTML(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "fixtures", "layouts.md")
	source, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if !rawHTMLPresent(source) {
		t.Fatal("layouts fixture does not exercise raw HTML trust")
	}

	slides, err := renderSlidesWithOptions(source, filepath.Dir(fixturePath), nil, renderOptions{allowRawHTML: true})
	if err != nil {
		t.Fatalf("renderSlidesWithOptions() error: %v", err)
	}
	if len(slides) != 3 {
		t.Fatalf("renderSlidesWithOptions() returned %d slides, want 3", len(slides))
	}
	if html := string(slides[0]); strings.Count(html, `class="column"`) != 2 || !strings.Contains(html, `src="data:image/png;base64,`) {
		t.Fatalf("two-column fixture slide did not preserve columns and embedded media: %s", html)
	}
	if html := string(slides[1]); strings.Count(html, `class="column"`) != 3 {
		t.Fatalf("three-column fixture slide did not preserve three columns: %s", html)
	}
	if html := string(slides[2]); !strings.Contains(html, "<mark>highlighted text</mark>") {
		t.Fatalf("inline HTML fixture slide was not preserved: %s", html)
	}
}

func TestExternalMediaFixtureCoversTrustBoundary(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "fixtures", "external-media", "deck.md")
	source, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	deckDirectory := filepath.Dir(fixturePath)

	if got, want := externalMediaReferences(source, deckDirectory), []string{
		"../assets/rendering-flow.png",
		"https://example.com/md-present-remote-image.png",
		"https://example.com/md-present-remote-video.mp4",
	}; !slices.Equal(got, want) {
		t.Fatalf("externalMediaReferences() = %#v, want %#v", got, want)
	}

	slides, err := renderSlides(source, deckDirectory)
	if err != nil {
		t.Fatalf("renderSlides() error: %v", err)
	}
	if len(slides) != 2 {
		t.Fatalf("renderSlides() returned %d slides, want 2", len(slides))
	}
	if html := string(slides[0]); !strings.Contains(html, `src="data:image/png;base64,`) {
		t.Fatalf("outside-deck image was not embedded: %s", html)
	}
	if html := string(slides[1]); !strings.Contains(html, `src="https://example.com/md-present-remote-image.png"`) || !strings.Contains(html, `<video controls preload="metadata" src="https://example.com/md-present-remote-video.mp4"`) {
		t.Fatalf("remote media was not kept remote: %s", html)
	}
}

func TestFenceSeparatorsFixturePreservesFencedContent(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "fence-separators.md"))
	if err != nil {
		t.Fatal(err)
	}

	slides, err := renderSlides(source, "")
	if err != nil {
		t.Fatalf("renderSlides() error: %v", err)
	}
	if len(slides) != 2 {
		t.Fatalf("renderSlides() returned %d slides, want 2", len(slides))
	}
	if html := string(slides[0]); !strings.Contains(html, "This separator is code") || !strings.Contains(html, "Still nested code.") {
		t.Fatalf("first slide did not preserve fenced content: %s", html)
	}
	if html := string(slides[1]); !strings.Contains(html, "Second real slide") || !strings.Contains(html, "&lt;slide&gt;") {
		t.Fatalf("second slide did not render safe unknown code: %s", html)
	}
}

func TestRenderSlidesRejectsEmptyDeck(t *testing.T) {
	if _, err := renderSlides([]byte(" \n---\n "), ""); err == nil {
		t.Fatal("renderSlides() accepted an empty deck")
	}
}

func TestRenderSlidesWarnsAboutUnterminatedFencedCodeBlock(t *testing.T) {
	for _, fence := range []string{"```", "~~~"} {
		t.Run(fence, func(t *testing.T) {
			var warnings bytes.Buffer
			source := []byte("# First\n\n" + fence + "text\n---\n# Still code")

			slides, err := renderSlidesWithWarnings(source, "", &warnings)
			if err != nil {
				t.Fatalf("renderSlidesWithWarnings() error: %v", err)
			}
			if len(slides) != 1 {
				t.Fatalf("renderSlidesWithWarnings() returned %d slides, want 1", len(slides))
			}
			want := "md-present: warning: unterminated fenced code block continues to the end of the document\n"
			if got := warnings.String(); got != want {
				t.Fatalf("warnings = %q, want %q", got, want)
			}
		})
	}
}

func TestUnterminatedFenceFixtureWarnsAndPreservesGoldmarkSemantics(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "unterminated-fence.md"))
	if err != nil {
		t.Fatal(err)
	}

	var warnings bytes.Buffer
	slides, err := renderSlidesWithWarnings(source, "", &warnings)
	if err != nil {
		t.Fatalf("renderSlidesWithWarnings() error: %v", err)
	}
	if len(slides) != 1 {
		t.Fatalf("renderSlidesWithWarnings() returned %d slides, want 1", len(slides))
	}
	if !strings.Contains(string(slides[0]), "This is code, not a second slide") {
		t.Fatal("unterminated fence fixture did not remain code")
	}
	if got := warnings.String(); got != "md-present: warning: unterminated fenced code block continues to the end of the document\n" {
		t.Fatalf("warnings = %q", got)
	}
}

func TestRenderSlidesDoesNotWarnAboutTerminatedFencedCodeBlock(t *testing.T) {
	var warnings bytes.Buffer
	if _, err := renderSlidesWithWarnings([]byte("```text\ncode\n```"), "", &warnings); err != nil {
		t.Fatalf("renderSlidesWithWarnings() error: %v", err)
	}
	if got := warnings.String(); got != "" {
		t.Fatalf("warnings = %q, want none", got)
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
