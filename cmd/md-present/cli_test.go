package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfirmExternalMedia(t *testing.T) {
	for _, test := range []struct {
		name    string
		input   string
		trusted bool
	}{
		{name: "yes", input: "yes\n", trusted: true},
		{name: "short yes", input: "Y\n", trusted: true},
		{name: "default no", input: "\n", trusted: false},
		{name: "no", input: "no\n", trusted: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			trusted, err := confirmExternalMedia(strings.NewReader(test.input), &output, "/private/slides.md", []string{"/private/video.mp4", "https://example.com/image.png"})
			if err != nil {
				t.Fatalf("confirmExternalMedia() error: %v", err)
			}
			if trusted != test.trusted {
				t.Fatalf("confirmExternalMedia() = %v, want %v", trusted, test.trusted)
			}
			for _, expected := range []string{"includes external media", "/private/video.mp4", "https://example.com/image.png", "Trust this file"} {
				if !strings.Contains(output.String(), expected) {
					t.Errorf("prompt omitted %q: %s", expected, output.String())
				}
			}
		})
	}
}

func TestConfirmRawHTML(t *testing.T) {
	for _, test := range []struct {
		name    string
		input   string
		trusted bool
	}{
		{name: "yes", input: "yes\n", trusted: true},
		{name: "short yes", input: "Y\n", trusted: true},
		{name: "default no", input: "\n", trusted: false},
		{name: "no", input: "no\n", trusted: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			trusted, err := confirmRawHTML(strings.NewReader(test.input), &output, "/private/slides.md")
			if err != nil {
				t.Fatalf("confirmRawHTML() error: %v", err)
			}
			if trusted != test.trusted {
				t.Fatalf("confirmRawHTML() = %v, want %v", trusted, test.trusted)
			}
			for _, expected := range []string{"includes raw HTML", "change presentation behavior", "Trust this file"} {
				if !strings.Contains(output.String(), expected) {
					t.Errorf("prompt omitted %q: %s", expected, output.String())
				}
			}
		})
	}
}

func TestReadConfirmationFromNonTerminalInput(t *testing.T) {
	answer, err := readConfirmation(strings.NewReader(" Yes\n"), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("readConfirmation() error: %v", err)
	}
	if answer != "yes" {
		t.Fatalf("readConfirmation() = %q, want %q", answer, "yes")
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name                   string
		args                   []string
		wantFile               string
		wantNoOpen             bool
		wantAllowExternalMedia bool
		wantAllowRawHTML       bool
		wantMCPPort            int
		wantAction             cliAction
		wantError              string
	}{
		{name: "file", args: []string{"slides.md"}, wantFile: "slides.md", wantAction: actionRun},
		{name: "no open", args: []string{"--no-open", "slides.md"}, wantFile: "slides.md", wantNoOpen: true, wantAction: actionRun},
		{name: "allow external media", args: []string{"--allow-external-media", "slides.md"}, wantFile: "slides.md", wantAllowExternalMedia: true, wantAction: actionRun},
		{name: "allow raw HTML", args: []string{"--allow-raw-html", "slides.md"}, wantFile: "slides.md", wantAllowRawHTML: true, wantAction: actionRun},
		{name: "option after file", args: []string{"slides.md", "--no-open"}, wantFile: "slides.md", wantNoOpen: true, wantAction: actionRun},
		{name: "dash filename", args: []string{"--", "-slides.md"}, wantFile: "-slides.md", wantAction: actionRun},
		{name: "mcp filename", args: []string{"--", "mcp"}, wantFile: "mcp", wantAction: actionRun},
		{name: "help", args: []string{"--help"}, wantAction: actionHelp},
		{name: "short help", args: []string{"-h"}, wantAction: actionHelp},
		{name: "version", args: []string{"--version"}, wantAction: actionVersion},
		{name: "missing", wantError: "missing Markdown file"},
		{name: "extra", args: []string{"one.md", "two.md"}, wantError: "unexpected argument"},
		{name: "unsupported", args: []string{"--theme", "slides.md"}, wantError: "unsupported option"},
		{name: "mcp install", args: []string{"mcp", "install"}, wantMCPPort: defaultMCPPort, wantAction: actionMCPInstall},
		{name: "mcp serve port", args: []string{"mcp", "serve", "--port", "49321"}, wantMCPPort: 49321, wantAction: actionMCPServe},
		{name: "mcp uninstall", args: []string{"mcp", "uninstall"}, wantMCPPort: defaultMCPPort, wantAction: actionMCPUninstall},
		{name: "mcp missing command", args: []string{"mcp"}, wantError: "missing MCP command"},
		{name: "mcp unsupported command", args: []string{"mcp", "start"}, wantError: "unsupported MCP command"},
		{name: "mcp invalid port", args: []string{"mcp", "serve", "--port", "0"}, wantError: "invalid MCP port"},
		{name: "mcp missing port", args: []string{"mcp", "install", "--port"}, wantError: "missing value"},
		{name: "mcp duplicate port", args: []string{"mcp", "serve", "--port", "38473", "--port", "38474"}, wantError: "only once"},
		{name: "mcp uninstall port", args: []string{"mcp", "uninstall", "--port", "38473"}, wantError: "not supported"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config, action, err := parseArgs(test.args)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("parseArgs() error = %v, want containing %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseArgs() unexpected error: %v", err)
			}
			if action != test.wantAction || config.markdownFile != test.wantFile || config.noOpen != test.wantNoOpen || config.allowExternalMedia != test.wantAllowExternalMedia || config.allowRawHTML != test.wantAllowRawHTML || config.mcpPort != test.wantMCPPort {
				t.Fatalf("parseArgs() = (%+v, %v), want file %q, no-open %v, allow-external-media %v, allow-raw-html %v, MCP port %d, action %v", config, action, test.wantFile, test.wantNoOpen, test.wantAllowExternalMedia, test.wantAllowRawHTML, test.wantMCPPort, test.wantAction)
			}
		})
	}
}

func TestRunRejectsUntrustedRawHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slides.md")
	if err := os.WriteFile(path, []byte("<div>Trusted content</div>\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := run([]string{"--no-open", path}, strings.NewReader("\n"), &stdout, &stderr); code != 1 {
		t.Fatalf("run() = %d, want 1; stderr:\n%s", code, stderr.String())
	}
	for _, expected := range []string{"includes raw HTML", "raw HTML was not trusted", "--allow-raw-html"} {
		if !strings.Contains(stderr.String(), expected) {
			t.Errorf("run() stderr omitted %q:\n%s", expected, stderr.String())
		}
	}
	if stdout.Len() != 0 {
		t.Errorf("run() stdout = %q, want empty", stdout.String())
	}
}

func TestResolveMarkdownPath(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "slides.md")
	if err := os.WriteFile(file, []byte("# Hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveMarkdownPath(file)
	if err != nil {
		t.Fatalf("resolveMarkdownPath() unexpected error: %v", err)
	}
	wantResolved, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != wantResolved {
		t.Fatalf("resolveMarkdownPath() = %q, want %q", resolved, wantResolved)
	}

	if _, err := resolveMarkdownPath(directory); err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("directory error = %v", err)
	}
	if _, err := resolveMarkdownPath(filepath.Join(directory, "missing.md")); err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("missing file error = %v", err)
	}
}
