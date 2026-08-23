package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var version = "0.2.2"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	config, action, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "md-present: %v\n", err)
		fmt.Fprintln(stderr, "Try 'md-present --help' for usage.")
		return 2
	}

	switch action {
	case actionHelp:
		printHelp(stdout)
		return 0
	case actionVersion:
		fmt.Fprintf(stdout, "md-present %s\n", version)
		return 0
	}

	path, err := resolveMarkdownPath(config.markdownFile)
	if err != nil {
		fmt.Fprintf(stderr, "md-present: %v\n", err)
		return 1
	}

	source, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "md-present: read %q: %v\n", config.markdownFile, err)
		return 1
	}

	slides, err := renderSlides(source, filepath.Dir(path))
	if err != nil {
		fmt.Fprintf(stderr, "md-present: render presentation: %v\n", err)
		return 1
	}

	if err := servePresentation(path, slides, config.noOpen, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "md-present: %v\n", err)
		return 1
	}
	return 0
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `Usage: md-present [--no-open] <markdown-file>

Render a Markdown file as a local browser presentation.

Options:
  --no-open  Start the server without opening a browser
  -h, --help Show this help
  --version  Show the version`)
}
