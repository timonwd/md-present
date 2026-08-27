package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	maxMCPPresentations = 8
	maxMCPRequestBytes  = 64 << 10
)

type presentFileInput struct {
	Path               string `json:"path" jsonschema:"absolute path to the Markdown file to present"`
	AllowExternalMedia bool   `json:"allow_external_media,omitempty" jsonschema:"allow remote media and local media outside the Markdown file's directory only after explicit user approval"`
	AllowRawHTML       bool   `json:"allow_raw_html,omitempty" jsonschema:"render CommonMark raw HTML only after explicit user approval"`
}

type presentFileOutput struct {
	PresentationURL  string   `json:"presentation_url,omitempty" jsonschema:"loopback URL of the browser presentation when it was opened"`
	Error            string   `json:"error,omitempty" jsonschema:"error explaining why the presentation did not open"`
	ApprovalRequired bool     `json:"approval_required,omitempty" jsonschema:"whether raw HTML or external media must be approved before the presentation can open"`
	ExternalMedia    []string `json:"external_media,omitempty" jsonschema:"remote or outside-directory media references requiring explicit user approval"`
	RawHTML          bool     `json:"raw_html,omitempty" jsonschema:"whether raw HTML requires explicit user approval"`
	Warnings         []string `json:"warnings,omitempty" jsonschema:"rendering warnings detected before the presentation opened"`
}

type filePresenter interface {
	present(context.Context, presentFileInput) (presentFileOutput, error)
}

type mcpPresentationManager struct {
	ctx    context.Context
	stderr io.Writer
	slots  chan struct{}
	start  func(context.Context, string, []template.HTML, renderOptions, bool, io.Writer, io.Writer) (*runningPresentation, error)
}

func newMCPPresentationManager(ctx context.Context, stderr io.Writer) *mcpPresentationManager {
	if stderr == nil {
		stderr = io.Discard
	}
	return &mcpPresentationManager{
		ctx:    ctx,
		stderr: stderr,
		slots:  make(chan struct{}, maxMCPPresentations),
		start:  startPresentation,
	}
}

func (m *mcpPresentationManager) present(_ context.Context, input presentFileInput) (presentFileOutput, error) {
	if !filepath.IsAbs(input.Path) {
		return presentFileOutput{}, fmt.Errorf("path must be absolute")
	}
	path, err := resolveMarkdownPath(input.Path)
	if err != nil {
		return presentFileOutput{}, err
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return presentFileOutput{}, fmt.Errorf("read %q: %w", input.Path, err)
	}
	requiresRawHTMLApproval := rawHTMLPresent(source) && !input.AllowRawHTML
	references := externalMediaReferences(source, filepath.Dir(path))
	requiresExternalMediaApproval := len(references) > 0 && !input.AllowExternalMedia
	if requiresRawHTMLApproval || requiresExternalMediaApproval {
		var requirements []string
		if requiresRawHTMLApproval {
			requirements = append(requirements, "allow_raw_html is required because the presentation contains raw HTML")
		}
		if requiresExternalMediaApproval {
			requirements = append(requirements, "allow_external_media is required because the presentation references external media")
		} else {
			references = nil
		}
		return presentFileOutput{
			Error:            strings.Join(requirements, "; "),
			ApprovalRequired: true,
			ExternalMedia:    references,
			RawHTML:          requiresRawHTMLApproval,
		}, nil
	}

	rendering := renderOptions{allowRawHTML: input.AllowRawHTML}
	var warnings bytes.Buffer
	slides, err := renderSlidesWithOptions(source, filepath.Dir(path), &warnings, rendering)
	if err != nil {
		return presentFileOutput{}, fmt.Errorf("render presentation: %w", err)
	}

	select {
	case m.slots <- struct{}{}:
	case <-m.ctx.Done():
		return presentFileOutput{}, fmt.Errorf("MCP server is shutting down")
	default:
		return presentFileOutput{}, fmt.Errorf("too many presentations are already running; close a presentation tab and retry")
	}

	presentation, err := m.start(m.ctx, path, slides, rendering, true, nil, m.stderr)
	if err != nil {
		<-m.slots
		return presentFileOutput{}, err
	}
	go func() {
		if err := presentation.wait(); err != nil {
			fmt.Fprintf(m.stderr, "Error: MCP presentation stopped: %v\n", err)
		}
		<-m.slots
	}()

	return presentFileOutput{
		PresentationURL: presentation.url,
		Warnings:        warningLines(warnings.String()),
	}, nil
}

func warningLines(value string) []string {
	var warnings []string
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			warnings = append(warnings, line)
		}
	}
	return warnings
}

func newMCPHandler(presenter filePresenter, port int) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{
		Name:        "md-present",
		Title:       "md-present",
		Description: "Open local Markdown files as browser presentations.",
		Version:     version,
	}, nil)
	destructive := false
	openWorld := true
	mcp.AddTool(server, &mcp.Tool{
		Name:        "present_file",
		Title:       "Present Markdown File",
		Description: "Open a local Markdown file as an md-present browser presentation. The file's directory is the root for relative media. Missing raw-HTML or external-media trust flags return a structured tool error; show raw_html and every external_media reference to the user, then retry with allow_raw_html or allow_external_media only after approval.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: &destructive,
			OpenWorldHint:   &openWorld,
			ReadOnlyHint:    false,
			IdempotentHint:  false,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input presentFileInput) (*mcp.CallToolResult, presentFileOutput, error) {
		output, err := presenter.present(ctx, input)
		if output.ApprovalRequired && err == nil {
			return &mcp.CallToolResult{IsError: true}, output, nil
		}
		return nil, output, err
	})

	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:                    true,
			JSONResponse:                 true,
			MaxRequestBodyBytes:          maxMCPRequestBytes,
			PropagateRequestCancellation: true,
		},
	)
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	return loopbackRequestProtection(port, mux)
}

func loopbackRequestProtection(port int, next http.Handler) http.Handler {
	expectedPort := fmt.Sprint(port)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedLoopbackHost(r.Host, expectedPort) {
			http.Error(w, "invalid Host header", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !allowedLoopbackOrigin(origin, expectedPort) {
			http.Error(w, "cross-origin request rejected", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func allowedLoopbackHost(value, expectedPort string) bool {
	host, port, err := net.SplitHostPort(value)
	return err == nil && port == expectedPort && (host == "127.0.0.1" || host == "localhost")
}

func allowedLoopbackOrigin(value, expectedPort string) bool {
	origin, err := url.Parse(value)
	if err != nil || origin.Scheme != "http" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return false
	}
	return allowedLoopbackHost(origin.Host, expectedPort)
}

func serveMCP(port int, stdout, stderr io.Writer) error {
	listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("listen on 127.0.0.1:%d: %w", port, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	manager := newMCPPresentationManager(ctx, stderr)
	server := &http.Server{
		Handler:           newMCPHandler(manager, port),
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(listener)
	}()

	fmt.Fprintf(stdout, "http://127.0.0.1:%d/mcp\n", port)
	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownServer(server)
		err := <-serverErrors
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	}
}
