package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeckThemeSeparatesFrontMatterAndBuildsSafeCSS(t *testing.T) {
	directory := t.TempDir()
	themePath := filepath.Join(directory, "brand.json")
	if err := os.WriteFile(themePath, []byte(`{
  "colors": {"canvas":"#f4f1ea", "stage":"#fffaf0", "ink":"#202020", "accent":"#a03d2f"},
  "darkColors": {"canvas":"#15110f", "stage":"#231b17", "ink":"#fff7ed", "accent":"#ffb4a8"},
  "fontFamily": "Avenir Next, Arial, sans-serif",
  "headingFontFamily": "Avenir Next Condensed, Arial, sans-serif",
  "headingWeight": "semibold",
  "headingLetterSpacing": "-0.04em",
  "headingCase": "uppercase",
  "headingAccent": "#a03d2f",
  "contentGutter": "2rem",
  "radius": "0.5rem",
  "shadow": "subtle",
  "tableHeader": "#efe3cf",
  "tableStripe": "#fbf3e6",
  "codeBackground": "#f4ead8",
  "quoteAccent": "#a03d2f",
  "linkColor": "#8c3025",
  "coverAlignment": "center",
  "coverBackground": "#f1ddc0",
  "coverInk": "#202020",
  "layout": "editorial",
  "mermaidNode": "#d68072",
  "mermaidEdge": "#8c3025",
  "mermaidLabel": "#202020",
  "footer": "Acme · Internal"
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	source := []byte("---\ntheme: brand\n---\n# Themed\n")
	body, path, err := parseDeckTheme(source, directory)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "# Themed\n" || path != themePath {
		t.Fatalf("parseDeckTheme() = (%q, %q), want body and %q", body, path, themePath)
	}
	css, loadedPath, err := deckThemeCSS(source, directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--canvas:#f4f1ea", "--accent:#a03d2f", "--theme-font-family:Avenir Next, Arial, sans-serif", "--theme-heading-font-family:Avenir Next Condensed, Arial, sans-serif", "--theme-heading-weight:600", "--theme-heading-letter-spacing:-0.04em", "--theme-heading-case:uppercase", "--theme-radius:0.5rem", "--theme-table-header:#efe3cf", "--theme-cover-align:center", "--theme-layout-scale:0.94", "--theme-mermaid-node:#d68072", "@media (prefers-color-scheme: dark)", "--accent:#ffb4a8"} {
		if !strings.Contains(css, want) {
			t.Errorf("theme CSS omitted %q: %s", want, css)
		}
	}
	if loadedPath != themePath {
		t.Errorf("loaded theme path = %q, want %q", loadedPath, themePath)
	}
	loaded, err := loadDeckTheme(source, directory)
	if err != nil || loaded.footer != "Acme · Internal" {
		t.Fatalf("loadDeckTheme() = (%#v, %v)", loaded, err)
	}
	slides, err := renderSlides(source, directory)
	if err != nil || len(slides) != 1 || strings.Contains(string(slides[0]), "theme: brand") || !strings.Contains(string(slides[0]), "Themed") {
		t.Fatalf("renderSlides() = (%q, %v)", slides, err)
	}
}

func TestDeckThemeRejectsUnsafeValuesAndUnknownFields(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "unsafe.json")
	if err := os.WriteFile(path, []byte(`{"colors":{"accent":"red; background:url(https://example.com)"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := deckThemeCSS([]byte("---\ntheme: unsafe\n---\n# Slide\n"), directory)
	if err == nil || !strings.Contains(err.Error(), "hexadecimal") {
		t.Fatalf("unsafe color error = %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"css":"body { display: none; }"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = deckThemeCSS([]byte("---\ntheme: unsafe\n---\n# Slide\n"), directory)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown theme field error = %v", err)
	}
}

func TestDeckThemeFindsBareNameInUserThemeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	themePath := filepath.Join(home, ".md-present", "themes", "acme.json")
	if err := os.MkdirAll(filepath.Dir(themePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(themePath, []byte(`{"colors":{"accent":"#a03d2f"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	css, path, err := deckThemeCSS([]byte("---\ntheme: acme\n---\n# Slide\n"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if path != themePath || !strings.Contains(css, "--accent:#a03d2f") {
		t.Fatalf("deckThemeCSS() = (%q, %q), want user theme %q", css, path, themePath)
	}
}

func TestDeckThemeAcceptsExtensionInFrontMatter(t *testing.T) {
	directory := t.TempDir()
	themePath := filepath.Join(directory, "acme.json")
	if err := os.WriteFile(themePath, []byte(`{"colors":{"accent":"#a03d2f"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, path, err := deckThemeCSS([]byte("---\ntheme: acme.json\n---\n# Slide\n"), directory)
	if err != nil || path != themePath {
		t.Fatalf("deckThemeCSS() = (%q, %v), want %q", path, err, themePath)
	}
}
