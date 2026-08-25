package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

const mcpLaunchAgentLabel = "com.timonwd.md-present.mcp"

type launchctlRunner func(...string) ([]byte, error)

func installMCPLaunchAgent(port int, stdout io.Writer) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("LaunchAgent installation is supported only on macOS")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	executable, err := launchAgentExecutable(os.Args[0])
	if err != nil {
		return err
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", mcpLaunchAgentLabel+".plist")
	if _, err := os.Stat(plistPath); err == nil {
		return fmt.Errorf("already installed at %q; run 'md-present mcp uninstall' first", plistPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect LaunchAgent %q: %w", plistPath, err)
	}
	if err := checkMCPPortAvailable(port); err != nil {
		return err
	}
	return installMCPLaunchAgentAt(home, executable, os.Getuid(), port, systemLaunchctl, stdout)
}

func uninstallMCPLaunchAgent(stdout, stderr io.Writer) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("LaunchAgent removal is supported only on macOS")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	return uninstallMCPLaunchAgentAt(home, os.Getuid(), systemLaunchctl, stdout, stderr)
}

func launchAgentExecutable(argumentZero string) (string, error) {
	path, err := exec.LookPath(argumentZero)
	if err != nil {
		return "", fmt.Errorf("locate md-present executable: %w", err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve md-present executable: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("inspect md-present executable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("md-present executable is not an executable regular file: %q", path)
	}
	return path, nil
}

func checkMCPPortAvailable(port int) error {
	listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("MCP port %d is unavailable: %w", port, err)
	}
	if err := listener.Close(); err != nil {
		return fmt.Errorf("release MCP port %d: %w", port, err)
	}
	return nil
}

func installMCPLaunchAgentAt(home, executable string, uid, port int, run launchctlRunner, stdout io.Writer) error {
	launchAgentsDirectory := filepath.Join(home, "Library", "LaunchAgents")
	logsDirectory := filepath.Join(home, "Library", "Logs")
	plistPath := filepath.Join(launchAgentsDirectory, mcpLaunchAgentLabel+".plist")
	if _, err := os.Stat(plistPath); err == nil {
		return fmt.Errorf("already installed at %q; run 'md-present mcp uninstall' first", plistPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect LaunchAgent %q: %w", plistPath, err)
	}
	if err := os.MkdirAll(launchAgentsDirectory, 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents directory: %w", err)
	}
	if err := os.MkdirAll(logsDirectory, 0o755); err != nil {
		return fmt.Errorf("create logs directory: %w", err)
	}
	logPath := filepath.Join(logsDirectory, "md-present-mcp.log")
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("create MCP log %q: %w", logPath, err)
	}
	if err := logFile.Close(); err != nil {
		return fmt.Errorf("close MCP log %q: %w", logPath, err)
	}
	if err := os.Chmod(logPath, 0o600); err != nil {
		return fmt.Errorf("secure MCP log %q: %w", logPath, err)
	}

	contents := launchAgentPlist(executable, logPath, port)
	file, err := os.OpenFile(plistPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create LaunchAgent %q: %w", plistPath, err)
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		_ = os.Remove(plistPath)
		return fmt.Errorf("write LaunchAgent %q: %w", plistPath, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(plistPath)
		return fmt.Errorf("close LaunchAgent %q: %w", plistPath, err)
	}

	domain := fmt.Sprintf("gui/%d", uid)
	if output, err := run("bootstrap", domain, plistPath); err != nil {
		_ = os.Remove(plistPath)
		return fmt.Errorf("launchctl bootstrap: %w%s", err, commandOutputSuffix(output))
	}

	fmt.Fprintf(stdout, "Installed md-present MCP LaunchAgent.\nEndpoint: http://127.0.0.1:%d/mcp\n", port)
	return nil
}

func uninstallMCPLaunchAgentAt(home string, uid int, run launchctlRunner, stdout, stderr io.Writer) error {
	plistPath := filepath.Join(home, "Library", "LaunchAgents", mcpLaunchAgentLabel+".plist")
	if _, err := os.Stat(plistPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("not installed at %q", plistPath)
		}
		return fmt.Errorf("inspect LaunchAgent %q: %w", plistPath, err)
	}

	domain := fmt.Sprintf("gui/%d", uid)
	if output, err := run("bootout", domain, plistPath); err != nil {
		service := domain + "/" + mcpLaunchAgentLabel
		if _, printErr := run("print", service); printErr == nil {
			return fmt.Errorf("launchctl bootout: %w%s", err, commandOutputSuffix(output))
		}
		fmt.Fprintf(stderr, "md-present: warning: LaunchAgent was not loaded: %v%s\n", err, commandOutputSuffix(output))
	}
	if err := os.Remove(plistPath); err != nil {
		return fmt.Errorf("remove LaunchAgent %q: %w", plistPath, err)
	}
	fmt.Fprintln(stdout, "Removed md-present MCP LaunchAgent.")
	return nil
}

func systemLaunchctl(args ...string) ([]byte, error) {
	return exec.Command("/bin/launchctl", args...).CombinedOutput()
}

func commandOutputSuffix(output []byte) string {
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return ""
	}
	return ": " + string(output)
}

func launchAgentPlist(executable, logPath string, port int) []byte {
	var output bytes.Buffer
	output.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>`)
	writeXMLEscaped(&output, mcpLaunchAgentLabel)
	output.WriteString(`</string>
  <key>ProgramArguments</key>
  <array>
    <string>`)
	writeXMLEscaped(&output, executable)
	output.WriteString(`</string>
    <string>mcp</string>
    <string>serve</string>
    <string>--port</string>
    <string>`)
	writeXMLEscaped(&output, strconv.Itoa(port))
	output.WriteString(`</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>LimitLoadToSessionType</key>
  <string>Aqua</string>
  <key>ProcessType</key>
  <string>Background</string>
  <key>Umask</key>
  <integer>63</integer>
  <key>StandardOutPath</key>
  <string>/dev/null</string>
  <key>StandardErrorPath</key>
  <string>`)
	writeXMLEscaped(&output, logPath)
	output.WriteString(`</string>
</dict>
</plist>
`)
	return output.Bytes()
}

func writeXMLEscaped(output io.Writer, value string) {
	_ = xml.EscapeText(output, []byte(value))
}
