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
			"allow_raw_html":       true,
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
	if input.Path != "/private/tmp/deck.md" || !input.AllowExternalMedia || !input.AllowRawHTML {
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

func TestMCPHandlerReturnsStructuredExternalMediaApprovalError(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "deck.md")
	remoteMedia := "https://example.com/image.png"
	if err := os.WriteFile(path, []byte("# Deck\n\n![Remote]("+remoteMedia+")"), 0o600); err != nil {
		t.Fatal(err)
	}

	manager := newMCPPresentationManager(t.Context(), io.Discard)
	manager.start = func(context.Context, string, []template.HTML, renderOptions, bool, io.Writer, io.Writer) (*runningPresentation, error) {
		t.Fatal("presentation started before external media approval")
		return nil, nil
	}
	server := newIPv4TestServer(t, func(port int) http.Handler { return newMCPHandler(manager, port) })
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

	callContext, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	result, err := session.CallTool(callContext, &mcp.CallToolParams{
		Name:      "present_file",
		Arguments: map[string]any{"path": path},
	})
	if err != nil {
		t.Fatalf("call present_file: %v", err)
	}
	if !result.IsError {
		t.Fatalf("approval requirement did not return a tool error: %+v", result.Content)
	}
	output, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured output = %#v", result.StructuredContent)
	}
	if !strings.Contains(fmt.Sprint(output["error"]), "allow_external_media is required") || output["approval_required"] != true || !strings.Contains(fmt.Sprint(output["external_media"]), remoteMedia) {
		t.Fatalf("approval output = %#v", output)
	}
	if len(result.Content) != 1 {
		t.Fatalf("approval content = %#v", result.Content)
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(textContent.Text, "allow_external_media is required") {
		t.Fatalf("approval content = %#v", result.Content)
	}

	rawRequest, err := http.NewRequestWithContext(callContext, http.MethodPost, server.URL+"/mcp", strings.NewReader(fmt.Sprintf(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"present_file","arguments":{"path":%q}}}`,
		path,
	)))
	if err != nil {
		t.Fatal(err)
	}
	rawRequest.Header.Set("Content-Type", "application/json")
	rawRequest.Header.Set("Accept", "application/json, text/event-stream")
	rawResponse, err := server.Client().Do(rawRequest)
	if err != nil {
		t.Fatalf("raw HTTP tool call: %v", err)
	}
	defer rawResponse.Body.Close()
	rawBody, err := io.ReadAll(rawResponse.Body)
	if err != nil {
		t.Fatalf("read raw HTTP tool response: %v", err)
	}
	if rawResponse.StatusCode != http.StatusOK || !strings.Contains(string(rawBody), `"isError":true`) || !strings.Contains(string(rawBody), "allow_external_media is required") {
		t.Fatalf("raw HTTP response = %d %s", rawResponse.StatusCode, rawBody)
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
	approval, err := manager.present(t.Context(), presentFileInput{Path: path})
	if err != nil {
		t.Fatalf("external media approval result: %v", err)
	}
	if !strings.Contains(approval.Error, "allow_external_media is required") || !approval.ApprovalRequired || !strings.Contains(fmt.Sprint(approval.ExternalMedia), "https://example.com/image.png") {
		t.Fatalf("external media approval = %+v", approval)
	}

	done := make(chan error, 1)
	done <- nil
	resolvedPath, err := resolveMarkdownPath(path)
	if err != nil {
		t.Fatal(err)
	}
	manager.start = func(_ context.Context, markdownPath string, slides []template.HTML, options renderOptions, open bool, stdout, _ io.Writer) (*runningPresentation, error) {
		if markdownPath != resolvedPath || len(slides) != 1 || options.allowRawHTML || !open || stdout != nil {
			t.Fatalf("startPresentation arguments = (%q, %d slides, options %+v, open %v, stdout %v)", markdownPath, len(slides), options, open, stdout)
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

func TestMCPPresentationManagerRequiresRawHTMLApproval(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "deck.md")
	if err := os.WriteFile(path, []byte(`<div class="columns">Trusted layout</div>`), 0o600); err != nil {
		t.Fatal(err)
	}

	manager := newMCPPresentationManager(t.Context(), io.Discard)
	manager.start = func(context.Context, string, []template.HTML, renderOptions, bool, io.Writer, io.Writer) (*runningPresentation, error) {
		t.Fatal("presentation started before raw HTML approval")
		return nil, nil
	}
	approval, err := manager.present(t.Context(), presentFileInput{Path: path})
	if err != nil {
		t.Fatalf("raw HTML approval result: %v", err)
	}
	if !strings.Contains(approval.Error, "allow_raw_html is required") || !approval.ApprovalRequired || !approval.RawHTML {
		t.Fatalf("raw HTML approval = %+v", approval)
	}

	done := make(chan error, 1)
	done <- nil
	manager.start = func(_ context.Context, _ string, slides []template.HTML, options renderOptions, _ bool, _ io.Writer, _ io.Writer) (*runningPresentation, error) {
		if !options.allowRawHTML || len(slides) != 1 || !strings.Contains(string(slides[0]), `class="columns"`) {
			t.Fatalf("trusted raw HTML start = (%d slides, options %+v)", len(slides), options)
		}
		return &runningPresentation{url: "http://127.0.0.1:49123/", done: done}, nil
	}
	output, err := manager.present(t.Context(), presentFileInput{Path: path, AllowRawHTML: true})
	if err != nil {
		t.Fatalf("approved raw HTML presentation error: %v", err)
	}
	if output.PresentationURL != "http://127.0.0.1:49123/" {
		t.Fatalf("presentation URL = %q", output.PresentationURL)
	}
}

func TestMCPPresentationManagerReportsAllTrustRequirements(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "deck.md")
	remoteMedia := "https://example.com/image.png"
	source := []byte("<div>\n\n![Remote](" + remoteMedia + ")\n\n</div>\n")
	if err := os.WriteFile(path, source, 0o600); err != nil {
		t.Fatal(err)
	}

	manager := newMCPPresentationManager(t.Context(), io.Discard)
	manager.start = func(context.Context, string, []template.HTML, renderOptions, bool, io.Writer, io.Writer) (*runningPresentation, error) {
		t.Fatal("presentation started before trust approval")
		return nil, nil
	}
	approval, err := manager.present(t.Context(), presentFileInput{Path: path})
	if err != nil {
		t.Fatalf("combined approval result: %v", err)
	}
	if !approval.ApprovalRequired || !approval.RawHTML || !strings.Contains(approval.Error, "allow_raw_html is required") || !strings.Contains(approval.Error, "allow_external_media is required") || !strings.Contains(fmt.Sprint(approval.ExternalMedia), remoteMedia) {
		t.Fatalf("combined approval = %+v", approval)
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
	manager.start = func(context.Context, string, []template.HTML, renderOptions, bool, io.Writer, io.Writer) (*runningPresentation, error) {
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
