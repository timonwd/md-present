package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantFile   string
		wantNoOpen bool
		wantAction cliAction
		wantError  string
	}{
		{name: "file", args: []string{"slides.md"}, wantFile: "slides.md", wantAction: actionRun},
		{name: "no open", args: []string{"--no-open", "slides.md"}, wantFile: "slides.md", wantNoOpen: true, wantAction: actionRun},
		{name: "option after file", args: []string{"slides.md", "--no-open"}, wantFile: "slides.md", wantNoOpen: true, wantAction: actionRun},
		{name: "dash filename", args: []string{"--", "-slides.md"}, wantFile: "-slides.md", wantAction: actionRun},
		{name: "help", args: []string{"--help"}, wantAction: actionHelp},
		{name: "short help", args: []string{"-h"}, wantAction: actionHelp},
		{name: "version", args: []string{"--version"}, wantAction: actionVersion},
		{name: "missing", wantError: "missing Markdown file"},
		{name: "extra", args: []string{"one.md", "two.md"}, wantError: "unexpected argument"},
		{name: "unsupported", args: []string{"--theme", "slides.md"}, wantError: "unsupported option"},
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
			if action != test.wantAction || config.markdownFile != test.wantFile || config.noOpen != test.wantNoOpen {
				t.Fatalf("parseArgs() = (%+v, %v), want file %q, no-open %v, action %v", config, action, test.wantFile, test.wantNoOpen, test.wantAction)
			}
		})
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
