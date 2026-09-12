package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxThemeBytes = 32 << 10

const themeFileExtension = ".json"

var (
	hexColor       = regexp.MustCompile(`^#[0-9a-fA-F]{3,4}([0-9a-fA-F]{2})?$`)
	fontFamily     = regexp.MustCompile(`^[A-Za-z0-9 ,.'"_-]+$`)
	spacing        = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]+)?(?:px|rem|em|vw|vh|cqw|cqh|%)$`)
	frontmatterEnd = []byte("\n---\n")
)

type themeFile struct {
	Colors               themeColors `json:"colors"`
	DarkColors           themeColors `json:"darkColors"`
	PrintColors          themeColors `json:"printColors"`
	FontFamily           string      `json:"fontFamily"`
	HeadingFontFamily    string      `json:"headingFontFamily"`
	HeadingWeight        string      `json:"headingWeight"`
	HeadingLetterSpacing string      `json:"headingLetterSpacing"`
	HeadingCase          string      `json:"headingCase"`
	HeadingAccent        string      `json:"headingAccent"`
	StageGutter          string      `json:"stageGutter"`
	ContentGutter        string      `json:"contentGutter"`
	Radius               string      `json:"radius"`
	Shadow               string      `json:"shadow"`
	TableHeader          string      `json:"tableHeader"`
	TableStripe          string      `json:"tableStripe"`
	CodeBackground       string      `json:"codeBackground"`
	QuoteAccent          string      `json:"quoteAccent"`
	LinkColor            string      `json:"linkColor"`
	CoverAlignment       string      `json:"coverAlignment"`
	CoverBackground      string      `json:"coverBackground"`
	CoverInk             string      `json:"coverInk"`
	Layout               string      `json:"layout"`
	MermaidNode          string      `json:"mermaidNode"`
	MermaidEdge          string      `json:"mermaidEdge"`
	MermaidLabel         string      `json:"mermaidLabel"`
	Footer               string      `json:"footer"`
}

type themeColors struct {
	Canvas string `json:"canvas"`
	Stage  string `json:"stage"`
	Ink    string `json:"ink"`
	Muted  string `json:"muted"`
	Line   string `json:"line"`
	Accent string `json:"accent"`
	Code   string `json:"code"`
}

// parseDeckTheme separates the optional front matter from the Markdown body.
// The deliberately small format keeps a theme reference readable without
// accepting arbitrary HTML, CSS, or JavaScript from the deck.
func parseDeckTheme(source []byte, deckDirectory string) ([]byte, string, error) {
	if !bytes.HasPrefix(source, []byte("---\n")) {
		return source, "", nil
	}
	end := bytes.Index(source[4:], frontmatterEnd)
	if end < 0 {
		return source, "", nil
	}
	frontmatter := string(source[4 : 4+end])
	body := source[4+end+len(frontmatterEnd):]
	var themeReference string
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) != "theme" {
			return nil, "", fmt.Errorf("unsupported document front matter %q", line)
		}
		if themeReference != "" {
			return nil, "", fmt.Errorf("document front matter defines theme more than once")
		}
		themeReference = strings.Trim(strings.TrimSpace(value), `"'`)
		if themeReference == "" {
			return nil, "", fmt.Errorf("document front matter theme is empty")
		}
	}
	if themeReference == "" {
		return nil, "", fmt.Errorf("document front matter requires a theme")
	}
	themePath, err := resolveThemePath(themeReference, deckDirectory)
	if err != nil {
		return nil, "", err
	}
	return body, themePath, nil
}

func resolveThemePath(reference, deckDirectory string) (string, error) {
	path := filepath.FromSlash(reference)
	if !strings.HasSuffix(strings.ToLower(path), themeFileExtension) {
		path += themeFileExtension
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	localPath := filepath.Join(deckDirectory, path)
	if _, err := os.Stat(localPath); err == nil || !isBareThemeName(reference) {
		return localPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return localPath, nil
	}
	globalPath := filepath.Join(home, ".md-present", "themes", path)
	if _, err := os.Stat(globalPath); err == nil {
		return globalPath, nil
	}
	return localPath, nil
}

func isBareThemeName(reference string) bool {
	return !strings.ContainsAny(reference, `/\\`) && reference != "." && reference != ".."
}

func deckThemeCSS(source []byte, deckDirectory string) (string, string, error) {
	theme, err := loadDeckTheme(source, deckDirectory)
	return theme.css, theme.path, err
}

type loadedTheme struct {
	css    string
	path   string
	footer string
}

func loadDeckTheme(source []byte, deckDirectory string) (loadedTheme, error) {
	_, path, err := parseDeckTheme(source, deckDirectory)
	if err != nil || path == "" {
		return loadedTheme{path: path}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return loadedTheme{path: path}, fmt.Errorf("read theme %q: %w", path, err)
	}
	if len(data) > maxThemeBytes {
		return loadedTheme{path: path}, fmt.Errorf("theme %q exceeds %d bytes", path, maxThemeBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var theme themeFile
	if err := decoder.Decode(&theme); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		if err == nil {
			err = fmt.Errorf("unexpected trailing content")
		}
		return loadedTheme{path: path}, fmt.Errorf("parse theme %q: %w", path, err)
	}
	if err := validateTheme(theme); err != nil {
		return loadedTheme{path: path}, fmt.Errorf("invalid theme %q: %w", path, err)
	}
	return loadedTheme{css: theme.css(), path: path, footer: theme.Footer}, nil
}

func validateTheme(theme themeFile) error {
	for _, colors := range []themeColors{theme.Colors, theme.DarkColors, theme.PrintColors} {
		for _, color := range []string{colors.Canvas, colors.Stage, colors.Ink, colors.Muted, colors.Line, colors.Accent, colors.Code} {
			if color != "" && !hexColor.MatchString(color) {
				return fmt.Errorf("colors must use hexadecimal values")
			}
		}
	}
	if theme.FontFamily != "" && !fontFamily.MatchString(theme.FontFamily) {
		return fmt.Errorf("fontFamily contains unsupported characters")
	}
	if theme.HeadingFontFamily != "" && !fontFamily.MatchString(theme.HeadingFontFamily) {
		return fmt.Errorf("headingFontFamily contains unsupported characters")
	}
	for _, value := range []string{theme.StageGutter, theme.ContentGutter, theme.Radius, theme.HeadingLetterSpacing} {
		if value != "" && !spacing.MatchString(value) {
			return fmt.Errorf("spacing must be a CSS length")
		}
	}
	for _, color := range []string{theme.HeadingAccent, theme.TableHeader, theme.TableStripe, theme.CodeBackground, theme.QuoteAccent, theme.LinkColor, theme.CoverBackground, theme.CoverInk, theme.MermaidNode, theme.MermaidEdge, theme.MermaidLabel} {
		if color != "" && !hexColor.MatchString(color) {
			return fmt.Errorf("theme colors must use hexadecimal values")
		}
	}
	if !oneOf(theme.HeadingWeight, "", "regular", "medium", "semibold", "bold") || !oneOf(theme.HeadingCase, "", "none", "uppercase", "lowercase", "capitalize") || !oneOf(theme.Shadow, "", "none", "subtle", "strong") || !oneOf(theme.CoverAlignment, "", "start", "center", "end") || !oneOf(theme.Layout, "", "default", "compact", "editorial") {
		return fmt.Errorf("theme uses an unsupported preset value")
	}
	if len(theme.Footer) > 240 || strings.ContainsAny(theme.Footer, "\r\n") {
		return fmt.Errorf("footer must be a single line of at most 240 characters")
	}
	return nil
}

func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func (theme themeFile) css() string {
	printColors := theme.Colors
	if theme.PrintColors != (themeColors{}) {
		printColors = theme.PrintColors
	}
	return ":root {" + theme.Colors.css() + declaration("--theme-font-family", theme.FontFamily) + declaration("--theme-heading-font-family", theme.HeadingFontFamily) + declaration("--theme-heading-weight", headingWeight(theme.HeadingWeight)) + declaration("--theme-heading-letter-spacing", theme.HeadingLetterSpacing) + declaration("--theme-heading-case", theme.HeadingCase) + declaration("--theme-heading-accent", theme.HeadingAccent) + declaration("--theme-stage-gutter", theme.StageGutter) + declaration("--theme-content-gutter", theme.ContentGutter) + declaration("--theme-radius", theme.Radius) + declaration("--theme-shadow", shadow(theme.Shadow)) + declaration("--theme-table-header", theme.TableHeader) + declaration("--theme-table-stripe", theme.TableStripe) + declaration("--theme-code-background", theme.CodeBackground) + declaration("--theme-quote-accent", theme.QuoteAccent) + declaration("--theme-link-color", theme.LinkColor) + declaration("--theme-cover-align", theme.CoverAlignment) + declaration("--theme-cover-background", theme.CoverBackground) + declaration("--theme-cover-ink", theme.CoverInk) + declaration("--theme-layout-scale", layoutScale(theme.Layout)) + declaration("--theme-heading-scale", headingScale(theme.Layout)) + declaration("--theme-mermaid-node", theme.MermaidNode) + declaration("--theme-mermaid-edge", theme.MermaidEdge) + declaration("--theme-mermaid-label", theme.MermaidLabel) + "}\n@media (prefers-color-scheme: dark) {:root {" + theme.DarkColors.css() + "}}\n@media print {:root {" + printColors.css() + "}}"
}

func headingWeight(value string) string {
	return map[string]string{"regular": "400", "medium": "500", "semibold": "600", "bold": "700"}[value]
}
func shadow(value string) string {
	return map[string]string{"none": "none", "subtle": "0 1rem 2.5rem rgb(20 25 38 / 10%)", "strong": "0 2.5rem 6rem rgb(20 25 38 / 24%)"}[value]
}
func layoutScale(value string) string {
	return map[string]string{"compact": "0.9", "editorial": "0.94"}[value]
}
func headingScale(value string) string {
	return map[string]string{"compact": "0.92", "editorial": "1.08"}[value]
}

func (colors themeColors) css() string {
	return declaration("--canvas", colors.Canvas) + declaration("--stage", colors.Stage) + declaration("--ink", colors.Ink) + declaration("--muted", colors.Muted) + declaration("--line", colors.Line) + declaration("--accent", colors.Accent) + declaration("--code", colors.Code)
}

func declaration(name, value string) string {
	if value == "" {
		return ""
	}
	return name + ":" + value + ";"
}
