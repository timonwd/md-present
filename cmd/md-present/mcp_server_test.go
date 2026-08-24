package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeFilePresenter struct {
	mu     sync.Mutex
	input  presentFileInput
	output presentFileOutput
	err    error
}

func (p *fakeFilePresenter) present(_ context.Context, input presentFileInput) (presentFileOutput, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.input = input
	return p.output, p.err
}

func TestMCPHandlerListsAndCallsPresentFile(t *testing.T) {
	presenter := &fakeFilePresenter{output: presentFileOutput{
		PresentationURL: "http://127.0.0.1:49123/",
		Warnings:        []string{"example warning"},
	}}
	server := newIPv4TestServer(t, func(port int) http.Handler { return newMCPHandler(presenter, port) })
	defer server.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "md-present-test", Version: "1.0.0"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:             server.URL + "/mcp",
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("list MCP tools: %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "present_file" {
		t.Fatalf("MCP tools = %+v", tools.Tools)
	}
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "present_file",
		Arguments: map[string]any{
			"path":                 "/private/tmp/deck.md",
			"allow_external_media": true,
		},
	})
	if err != nil {
		t.Fatalf("call present_file: %v", err)
	}
	if result.IsError {
		t.Fatalf("present_file returned tool error: %+v", result.Content)
	}
	presenter.mu.Lock()
	input := presenter.input
	presenter.mu.Unlock()
	if input.Path != "/private/tmp/deck.md" || !input.AllowExternalMedia {
		t.Fatalf("present_file input = %+v", input)
	}
	if !strings.Contains(fmt.Sprint(result.StructuredContent), presenter.output.PresentationURL) {
		t.Fatalf("present_file structured output = %#v", result.StructuredContent)
	}
}

func TestMCPHandlerRejectsUntrustedHostAndOrigin(t *testing.T) {
	handler := newMCPHandler(&fakeFilePresenter{}, defaultMCPPort)
	tests := []struct {
		name   string
		host   string
		origin string
	}{
		{name: "foreign host", host: "attacker.example:38473"},
		{name: "wrong port", host: "127.0.0.1:38474"},
		{name: "foreign origin", host: "127.0.0.1:38473", origin: "https://attacker.example"},
		{name: "loopback wrong port origin", host: "127.0.0.1:38473", origin: "http://127.0.0.1:38474"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:38473/mcp", strings.NewReader("{}"))
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", response.Code)
			}
		})
	}
}

func TestMCPPresentationManagerRequiresAbsolutePathAndMediaApproval(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "deck.md")
	if err := os.WriteFile(path, []byte("# Deck\n\n![Remote](https://example.com/image.png)"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := newMCPPresentationManager(t.Context(), io.Discard)
	if _, err := manager.present(t.Context(), presentFileInput{Path: "deck.md"}); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative path error = %v", err)
	}
	if _, err := manager.present(t.Context(), presentFileInput{Path: path}); err == nil || !strings.Contains(err.Error(), "https://example.com/image.png") {
		t.Fatalf("external media error = %v", err)
	}

	done := make(chan error, 1)
	done <- nil
	resolvedPath, err := resolveMarkdownPath(path)
	if err != nil {
		t.Fatal(err)
	}
	manager.start = func(_ context.Context, markdownPath string, slides []template.HTML, open bool, stdout, _ io.Writer) (*runningPresentation, error) {
		if markdownPath != resolvedPath || len(slides) != 1 || !open || stdout != nil {
			t.Fatalf("startPresentation arguments = (%q, %d slides, open %v, stdout %v)", markdownPath, len(slides), open, stdout)
		}
		return &runningPresentation{url: "http://127.0.0.1:49123/", done: done}, nil
	}
	output, err := manager.present(t.Context(), presentFileInput{Path: path, AllowExternalMedia: true})
	if err != nil {
		t.Fatalf("approved presentation error: %v", err)
	}
	if output.PresentationURL != "http://127.0.0.1:49123/" {
		t.Fatalf("presentation URL = %q", output.PresentationURL)
	}
}

func TestMCPPresentationManagerBoundsConcurrentPresentations(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "deck.md")
	if err := os.WriteFile(path, []byte("# Deck"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := newMCPPresentationManager(t.Context(), io.Discard)
	done := make(chan error)
	manager.start = func(context.Context, string, []template.HTML, bool, io.Writer, io.Writer) (*runningPresentation, error) {
		return &runningPresentation{url: "http://127.0.0.1:49123/", done: done}, nil
	}
	for range maxMCPPresentations {
		if _, err := manager.present(t.Context(), presentFileInput{Path: path}); err != nil {
			t.Fatalf("start presentation: %v", err)
		}
	}
	if _, err := manager.present(t.Context(), presentFileInput{Path: path}); err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("presentation limit error = %v", err)
	}
	close(done)
	deadline := time.Now().Add(time.Second)
	for len(manager.slots) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
}

func newIPv4TestServer(t *testing.T, handler func(int) http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	server := httptest.NewUnstartedServer(handler(port))
	server.Listener = listener
	server.Start()
	return server
}
