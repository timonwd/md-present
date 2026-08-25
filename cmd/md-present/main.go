package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var version = "0.3.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
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
	case actionMCPServe:
		if err := serveMCP(config.mcpPort, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "md-present: MCP server: %v\n", err)
			return 1
		}
		return 0
	case actionMCPInstall:
		if err := installMCPLaunchAgent(config.mcpPort, stdout); err != nil {
			fmt.Fprintf(stderr, "md-present: install MCP server: %v\n", err)
			return 1
		}
		return 0
	case actionMCPUninstall:
		if err := uninstallMCPLaunchAgent(stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "md-present: uninstall MCP server: %v\n", err)
			return 1
		}
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
	if references := externalMediaReferences(source, filepath.Dir(path)); len(references) > 0 && !config.allowExternalMedia {
		trusted, err := confirmExternalMedia(stdin, stderr, path, references)
		if err != nil {
			fmt.Fprintf(stderr, "md-present: confirm external media: %v\n", err)
			return 1
		}
		if !trusted {
			fmt.Fprintln(stderr, "md-present: external media was not trusted; rerun with --allow-external-media to opt in")
			return 1
		}
	}

	slides, err := renderSlidesWithWarnings(source, filepath.Dir(path), stderr)
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
	fmt.Fprintln(w, `Usage:
  md-present [--no-open] [--allow-external-media] <markdown-file>
  md-present mcp install [--port <port>]
  md-present mcp uninstall
  md-present mcp serve [--port <port>]

Render a Markdown file as a local browser presentation.

Options:
  --no-open               Start the server without opening a browser
  --allow-external-media  Trust remote and outside-deck media without prompting
  --port <port>            MCP loopback port (default: 38473)
  -h, --help              Show this help
  --version               Show the version`)
}

func confirmExternalMedia(input io.Reader, output io.Writer, markdownPath string, references []string) (bool, error) {
	fmt.Fprintf(output, "md-present: %q includes external media:\n", markdownPath)
	for _, reference := range references {
		fmt.Fprintf(output, "  - %s\n", reference)
	}
	fmt.Fprint(output, "Trust this file and allow its external media? [y/N] ")

	scanner := bufio.NewScanner(input)
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
