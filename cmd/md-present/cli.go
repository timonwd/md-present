package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type cliAction int

const (
	actionRun cliAction = iota
	actionHelp
	actionVersion
)

type cliConfig struct {
	markdownFile       string
	noOpen             bool
	allowExternalMedia bool
}

func parseArgs(args []string) (cliConfig, cliAction, error) {
	var config cliConfig
	var positional []string
	options := true

	for _, arg := range args {
		if options {
			switch arg {
			case "-h", "--help":
				return cliConfig{}, actionHelp, nil
			case "--version":
				return cliConfig{}, actionVersion, nil
			case "--no-open":
				config.noOpen = true
				continue
			case "--allow-external-media":
				config.allowExternalMedia = true
				continue
			case "--":
				options = false
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return cliConfig{}, actionRun, fmt.Errorf("unsupported option %q", arg)
			}
		}
		positional = append(positional, arg)
	}

	if len(positional) == 0 {
		return cliConfig{}, actionRun, fmt.Errorf("missing Markdown file")
	}
	if len(positional) > 1 {
		return cliConfig{}, actionRun, fmt.Errorf("unexpected argument %q", positional[1])
	}
	config.markdownFile = positional[0]
	return config, actionRun, nil
}

func resolveMarkdownPath(input string) (string, error) {
	abs, err := filepath.Abs(input)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", input, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %q", input)
		}
		return "", fmt.Errorf("resolve %q: %w", input, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("inspect %q: %w", input, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("expected a Markdown file, got directory: %q", input)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("expected a regular file: %q", input)
	}
	return resolved, nil
}
