package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
)

func TestSyntaxHighlightingHighlightsKnownLanguage(t *testing.T) {
	output := renderMarkdownWithSyntaxHighlighting(t, "```go\npackage main\n```\n")

	for _, expected := range []string{
		`<pre class="chroma"><code class="language-go">`,
		`<span class="kn">package</span>`,
		`<span class="nx">main</span>`,
		"</code></pre>",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("highlighted output missing %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, `style="`) {
		t.Errorf("highlighted output contains a CSP-incompatible inline style:\n%s", output)
	}
}

func TestSyntaxHighlightingFallsBackForUnknownLanguage(t *testing.T) {
	output := renderMarkdownWithSyntaxHighlighting(t, "```not-a-real-language\n<tag>& value\n```\n")
	want := "<pre><code class=\"language-not-a-real-language\">&lt;tag&gt;&amp; value\n</code></pre>\n"
	if output != want {
		t.Fatalf("unknown-language output:\n got: %q\nwant: %q", output, want)
	}
}

func TestSyntaxHighlightingFallsBackWithoutLanguage(t *testing.T) {
	output := renderMarkdownWithSyntaxHighlighting(t, "```\n<tag>& value\n```\n")
	want := "<pre><code>&lt;tag&gt;&amp; value\n</code></pre>\n"
	if output != want {
		t.Fatalf("language-less output:\n got: %q\nwant: %q", output, want)
	}
}

func TestSyntaxHighlightingKeepsRawHTMLDisabled(t *testing.T) {
	output := renderMarkdownWithSyntaxHighlighting(t, "<script>alert('unsafe')</script>\n\n```go\nvar payload = \"<script>\"\n```\n")

	if strings.Contains(output, "<script>") {
		t.Fatalf("output contains executable raw HTML:\n%s", output)
	}
	if !strings.Contains(output, "&lt;script&gt;") {
		t.Fatalf("highlighted code was not HTML-escaped:\n%s", output)
	}
}

func TestSyntaxHighlightingEscapesLanguageClass(t *testing.T) {
	output := renderMarkdownWithSyntaxHighlighting(t, "```unknown\"><script>\ncode\n```\n")

	if strings.Contains(output, "<script>") || strings.Contains(output, `class="language-unknown">`) {
		t.Fatalf("language info escaped its class attribute:\n%s", output)
	}
	if !strings.Contains(output, `class="language-unknown&quot;&gt;&lt;script&gt;"`) {
		t.Fatalf("escaped language class missing from output:\n%s", output)
	}
}

func renderMarkdownWithSyntaxHighlighting(t *testing.T, source string) string {
	t.Helper()
	markdown := goldmark.New(goldmark.WithExtensions(syntaxHighlighting))
	var output bytes.Buffer
	if err := markdown.Convert([]byte(source), &output); err != nil {
		t.Fatalf("render Markdown: %v", err)
	}
	return output.String()
}
