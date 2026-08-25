package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestLaunchAgentPlist(t *testing.T) {
	contents := launchAgentPlist("/Applications/A&B/<md-present>", "/Users/test/Library/Logs/md-present.log", 41234)
	if err := xml.Unmarshal(contents, new(any)); err != nil {
		t.Fatalf("launchAgentPlist() produced invalid XML: %v\n%s", err, contents)
	}
	for _, expected := range []string{
		mcpLaunchAgentLabel,
		"/Applications/A&amp;B/&lt;md-present&gt;",
		"<string>mcp</string>",
		"<string>serve</string>",
		"<string>41234</string>",
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
		"<key>Umask</key>",
	} {
		if !strings.Contains(string(contents), expected) {
			t.Errorf("LaunchAgent plist omitted %q:\n%s", expected, contents)
		}
	}
	if runtime.GOOS == "darwin" {
		command := exec.Command("/usr/bin/plutil", "-lint", "-")
		command.Stdin = bytes.NewReader(contents)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("plutil rejected LaunchAgent plist: %v: %s", err, output)
		}
	}
}

func TestLaunchAgentExecutablePreservesStableSymlink(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "md-present-0.2.7")
	link := filepath.Join(directory, "md-present")
	if err := os.WriteFile(target, []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	resolved, err := launchAgentExecutable(link)
	if err != nil {
		t.Fatalf("launchAgentExecutable() error: %v", err)
	}
	if resolved != link {
		t.Fatalf("launchAgentExecutable() = %q, want stable symlink %q", resolved, link)
	}
}

func TestInstallAndUninstallMCPLaunchAgent(t *testing.T) {
	home := t.TempDir()
	var calls [][]string
	run := func(args ...string) ([]byte, error) {
		calls = append(calls, slices.Clone(args))
		return nil, nil
	}
	var installOutput bytes.Buffer
	if err := installMCPLaunchAgentAt(home, "/opt/homebrew/bin/md-present", 501, 41234, run, &installOutput); err != nil {
		t.Fatalf("installMCPLaunchAgentAt() error: %v", err)
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", mcpLaunchAgentLabel+".plist")
	info, err := os.Stat(plistPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("LaunchAgent mode = %o, want 644", info.Mode().Perm())
	}
	logInfo, err := os.Stat(filepath.Join(home, "Library", "Logs", "md-present-mcp.log"))
	if err != nil {
		t.Fatal(err)
	}
	if logInfo.Mode().Perm() != 0o600 {
		t.Errorf("MCP log mode = %o, want 600", logInfo.Mode().Perm())
	}
	if len(calls) != 1 || !slices.Equal(calls[0], []string{"bootstrap", "gui/501", plistPath}) {
		t.Fatalf("install launchctl calls = %v", calls)
	}
	if !strings.Contains(installOutput.String(), "http://127.0.0.1:41234/mcp") {
		t.Fatalf("install output omitted endpoint: %s", installOutput.String())
	}

	var uninstallOutput bytes.Buffer
	var uninstallWarnings bytes.Buffer
	if err := uninstallMCPLaunchAgentAt(home, 501, run, &uninstallOutput, &uninstallWarnings); err != nil {
		t.Fatalf("uninstallMCPLaunchAgentAt() error: %v", err)
	}
	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Fatalf("LaunchAgent still exists after uninstall: %v", err)
	}
	if len(calls) != 2 || !slices.Equal(calls[1], []string{"bootout", "gui/501", plistPath}) {
		t.Fatalf("uninstall launchctl calls = %v", calls)
	}
	if uninstallWarnings.Len() != 0 || !strings.Contains(uninstallOutput.String(), "Removed") {
		t.Fatalf("uninstall output = %q, warnings = %q", uninstallOutput.String(), uninstallWarnings.String())
	}
}

func TestInstallMCPLaunchAgentRollsBackBootstrapFailure(t *testing.T) {
	home := t.TempDir()
	run := func(...string) ([]byte, error) { return []byte("service failed"), errors.New("exit status 5") }
	err := installMCPLaunchAgentAt(home, "/opt/homebrew/bin/md-present", 501, defaultMCPPort, run, new(bytes.Buffer))
	if err == nil || !strings.Contains(err.Error(), "service failed") {
		t.Fatalf("install error = %v", err)
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", mcpLaunchAgentLabel+".plist")
	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Fatalf("failed install left LaunchAgent behind: %v", err)
	}
}

func TestUninstallMCPLaunchAgentRemovesUnloadedPlist(t *testing.T) {
	home := t.TempDir()
	plistPath := filepath.Join(home, "Library", "LaunchAgents", mcpLaunchAgentLabel+".plist")
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plistPath, []byte("plist"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(...string) ([]byte, error) { return []byte("not loaded"), errors.New("exit status 3") }
	var warnings bytes.Buffer
	if err := uninstallMCPLaunchAgentAt(home, 501, run, new(bytes.Buffer), &warnings); err != nil {
		t.Fatalf("uninstall error: %v", err)
	}
	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Fatalf("uninstall left plist behind: %v", err)
	}
	if !strings.Contains(warnings.String(), "not loaded") {
		t.Fatalf("uninstall warning omitted launchctl output: %s", warnings.String())
	}
}

func TestUninstallMCPLaunchAgentKeepsPlistWhenServiceRemainsLoaded(t *testing.T) {
	home := t.TempDir()
	plistPath := filepath.Join(home, "Library", "LaunchAgents", mcpLaunchAgentLabel+".plist")
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plistPath, []byte("plist"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) ([]byte, error) {
		if args[0] == "print" {
			return []byte("loaded"), nil
		}
		return []byte("bootout denied"), errors.New("exit status 1")
	}
	err := uninstallMCPLaunchAgentAt(home, 501, run, new(bytes.Buffer), new(bytes.Buffer))
	if err == nil || !strings.Contains(err.Error(), "bootout denied") {
		t.Fatalf("uninstall error = %v", err)
	}
	if _, err := os.Stat(plistPath); err != nil {
		t.Fatalf("failed uninstall removed the plist: %v", err)
	}
}
