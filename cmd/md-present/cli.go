package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultMCPPort = 38473

type cliAction int

const (
	actionRun cliAction = iota
	actionHelp
	actionVersion
	actionMCPServe
	actionMCPInstall
	actionMCPUninstall
)

type cliConfig struct {
	markdownFile       string
	noOpen             bool
	allowExternalMedia bool
	mcpPort            int
}

func parseArgs(args []string) (cliConfig, cliAction, error) {
	if len(args) > 0 && args[0] == "mcp" {
		return parseMCPArgs(args[1:])
	}

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

func parseMCPArgs(args []string) (cliConfig, cliAction, error) {
	config := cliConfig{mcpPort: defaultMCPPort}
	if len(args) == 0 {
		return cliConfig{}, actionRun, fmt.Errorf("missing MCP command (expected install, serve, or uninstall)")
	}

	var action cliAction
	switch args[0] {
	case "install":
		action = actionMCPInstall
	case "serve":
		action = actionMCPServe
	case "uninstall":
		action = actionMCPUninstall
	case "-h", "--help":
		return cliConfig{}, actionHelp, nil
	default:
		return cliConfig{}, actionRun, fmt.Errorf("unsupported MCP command %q", args[0])
	}

	portSeen := false
	for index := 1; index < len(args); index++ {
		arg := args[index]
		if arg == "-h" || arg == "--help" {
			return cliConfig{}, actionHelp, nil
		}
		if arg != "--port" {
			return cliConfig{}, actionRun, fmt.Errorf("unsupported option %q", arg)
		}
		if action == actionMCPUninstall {
			return cliConfig{}, actionRun, fmt.Errorf("--port is not supported by MCP uninstall")
		}
		if portSeen {
			return cliConfig{}, actionRun, fmt.Errorf("--port may be specified only once")
		}
		portSeen = true
		index++
		if index == len(args) {
			return cliConfig{}, actionRun, fmt.Errorf("missing value for --port")
		}
		port, err := strconv.Atoi(args[index])
		if err != nil || port < 1 || port > 65535 {
			return cliConfig{}, actionRun, fmt.Errorf("invalid MCP port %q", args[index])
		}
		config.mcpPort = port
	}

	return config, action, nil
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
