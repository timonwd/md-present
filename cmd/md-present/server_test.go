package main

import (
	"bufio"
	"bytes"
	"context"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPresentationHandler(t *testing.T) {
	tracker := newTabTracker(func() {}, time.Second)
	presentation := newPresentationState([]template.HTML{`<h1>Expected slide</h1><pre><code class="language-mermaid">flowchart LR
A --&gt; B
</code></pre>`})
	handler := presentationHandler(presentation, tracker, io.Discard)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Expected slide") {
		t.Fatalf("GET / status %d, body:\n%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `class="fullscreen-button"`) {
		t.Fatal("GET / omitted fullscreen control")
	}
	if !strings.Contains(response.Body.String(), `class="overflow-warning"`) {
		t.Fatal("GET / omitted overflow warning")
	}
	if !strings.Contains(response.Body.String(), `class="overview-button"`) {
		t.Fatal("GET / omitted overview control")
	}
	if !strings.Contains(response.Body.String(), `aria-keyshortcuts="O"`) {
		t.Fatal("GET / omitted overview keyboard shortcut")
	}
	csp := response.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("GET / omitted Content-Security-Policy")
	}
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "'unsafe-inline'") || strings.Contains(csp, "'unsafe-eval'") {
		t.Errorf("GET / has unsafe script policy: %s", csp)
	}
	for _, expected := range []string{`class="language-mermaid"`, `/assets/mermaid.min.js`, `/assets/app.js`} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("GET / omitted %q", expected)
		}
	}

	for _, asset := range []string{"app.js", "mermaid.min.js", "mermaid.LICENSE.txt"} {
		assetRequest := httptest.NewRequest(http.MethodGet, "/assets/"+asset, nil)
		assetResponse := httptest.NewRecorder()
		handler.ServeHTTP(assetResponse, assetRequest)
		if assetResponse.Code != http.StatusOK {
			t.Errorf("GET /assets/%s status = %d", asset, assetResponse.Code)
		}
		if asset == "app.js" {
			body := assetResponse.Body.String()
			for _, expected := range []string{"requestFullscreen", "exitFullscreen", `securityLevel: "strict"`, `role", "alert"`, "language-mermaid", "overflowWarningDuration", "scrollBy", "toggleOverview", `role", "grid"`, `role", "gridcell"`} {
				if !strings.Contains(body, expected) {
					t.Errorf("GET /assets/app.js omitted %q", expected)
				}
			}
		} else if asset == "mermaid.min.js" && !strings.Contains(assetResponse.Body.String(), `globalThis["mermaid"]`) {
			t.Error("GET /assets/mermaid.min.js omitted Mermaid browser export")
		} else if asset == "mermaid.LICENSE.txt" && !strings.Contains(assetResponse.Body.String(), "The MIT License (MIT)") {
			t.Error("GET /assets/mermaid.LICENSE.txt omitted MIT license")
		}
	}

	missingRequest := httptest.NewRequest(http.MethodGet, "/source.md", nil)
	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, missingRequest)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("GET /source.md status = %d, want 404", missingResponse.Code)
	}
}

func TestPresentationHandlerReportsOverflow(t *testing.T) {
	presentation := newPresentationState([]template.HTML{"<h1>One</h1>", "<h1>Two</h1>"})
	var diagnostics bytes.Buffer
	handler := presentationHandler(presentation, newTabTracker(func() {}, time.Second), &diagnostics)
	bodies := []string{
		`{"revision":1,"slides":[2],"stageWidth":1280,"stageHeight":720}`,
		`{"revision":1,"slides":[2],"stageWidth":960,"stageHeight":540}`,
	}

	for _, body := range bodies {
		request := httptest.NewRequest(http.MethodPost, "/api/overflow", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "http://example.com")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("POST /api/overflow status = %d, want %d", response.Code, http.StatusNoContent)
		}
	}

	want := "md-present: warning: slide 2 exceeds the regular 16:9 slide area at 1280x720; scroll to view all content\n"
	if got := diagnostics.String(); got != want {
		t.Fatalf("overflow diagnostics = %q, want %q", got, want)
	}
}

func TestPresentationHandlerRejectsInvalidOverflowReport(t *testing.T) {
	presentation := newPresentationState([]template.HTML{"<h1>One</h1>"})
	handler := presentationHandler(presentation, newTabTracker(func() {}, time.Second), io.Discard)
	request := httptest.NewRequest(http.MethodPost, "/api/overflow", strings.NewReader(`{"revision":1,"slides":[1],"stageWidth":1280,"stageHeight":720}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://example.com")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin POST /api/overflow status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestPresentationHandlerStreamsReload(t *testing.T) {
	presentation := newPresentationState([]template.HTML{"<h1>Before</h1>"})
	server := httptest.NewServer(presentationHandler(presentation, newTabTracker(func() {}, time.Second), io.Discard))
	defer server.Close()

	response, err := server.Client().Get(server.URL + "/api/session?revision=1")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if contentType := response.Header.Get("Content-Type"); contentType != "text/event-stream" {
		t.Fatalf("session Content-Type = %q", contentType)
	}

	scanner := bufio.NewScanner(response.Body)
	for range 3 {
		if !scanner.Scan() {
			t.Fatalf("read ready event: %v", scanner.Err())
		}
	}
	presentation.update([]template.HTML{"<h1>After</h1>"})
	if !scanner.Scan() || scanner.Text() != "event: reload" {
		t.Fatalf("reload event line = %q, error = %v", scanner.Text(), scanner.Err())
	}
	if !scanner.Scan() || scanner.Text() != "data: 2" {
		t.Fatalf("reload data line = %q, error = %v", scanner.Text(), scanner.Err())
	}
}

func TestPresentationServerCancelsSessionStreamDuringShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	presentation := newPresentationState([]template.HTML{"<h1>Expected slide</h1>"})
	server := &http.Server{
		Handler:     presentationHandler(presentation, newTabTracker(cancel, time.Second), io.Discard),
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(listener)
	}()

	response, err := http.Get("http://" + listener.Addr().String() + "/api/session?revision=1")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown with an active session stream: %v", err)
	}
	<-serverErrors
}

func TestPresentationStatePublishesUpdates(t *testing.T) {
	presentation := newPresentationState([]template.HTML{"<h1>Before</h1>"})
	updates, revision, unsubscribe := presentation.subscribe()
	defer unsubscribe()
	if revision != 1 {
		t.Fatalf("initial revision = %d, want 1", revision)
	}
	if presentation.update([]template.HTML{"<h1>Before</h1>"}) {
		t.Fatal("identical slides triggered an update")
	}
	if !presentation.update([]template.HTML{"<h1>After</h1>"}) {
		t.Fatal("changed slides did not trigger an update")
	}
	select {
	case revision = <-updates:
		if revision != 2 {
			t.Fatalf("published revision = %d, want 2", revision)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("changed slides did not publish a revision")
	}
}

func TestRefreshPresentationKeepsLastValidRender(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "slides.md")
	if err := os.WriteFile(path, []byte("# Before"), 0o600); err != nil {
		t.Fatal(err)
	}
	presentation := newPresentationState([]template.HTML{"<h1>Before</h1>\n"})
	if err := os.WriteFile(path, []byte("# After"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := refreshPresentation(path, presentation); err != nil {
		t.Fatalf("refreshPresentation() error: %v", err)
	}
	slides, revision := presentation.snapshot()
	if revision != 2 || !strings.Contains(string(slides[0]), "After") {
		t.Fatalf("updated presentation = (%q, %d)", slides, revision)
	}

	if err := os.WriteFile(path, []byte("---"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := refreshPresentation(path, presentation); err == nil {
		t.Fatal("refreshPresentation() accepted an empty deck")
	}
	slides, revision = presentation.snapshot()
	if revision != 2 || !strings.Contains(string(slides[0]), "After") {
		t.Fatalf("invalid refresh replaced presentation = (%q, %d)", slides, revision)
	}
}

func TestRefreshPresentationDetectsLocalImageChanges(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "slides.md")
	imagePath := filepath.Join(directory, "image.png")
	if err := os.WriteFile(path, []byte("![Image](image.png)"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(imagePath, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	initial, err := renderSlides([]byte("![Image](image.png)"), directory)
	if err != nil {
		t.Fatal(err)
	}
	presentation := newPresentationState(initial)

	if err := os.WriteFile(imagePath, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := refreshPresentation(path, presentation); err != nil {
		t.Fatalf("refreshPresentation() error: %v", err)
	}
	slides, revision := presentation.snapshot()
	if revision != 2 || slices.Equal(slides, initial) {
		t.Fatalf("image refresh presentation = (%q, %d)", slides, revision)
	}
}

func TestTabTrackerShutsDownAfterLastDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tracker := newTabTracker(cancel, 10*time.Millisecond)

	first := tracker.connected()
	second := tracker.connected()
	first()
	select {
	case <-ctx.Done():
		t.Fatal("tracker stopped while a tab remained connected")
	case <-time.After(20 * time.Millisecond):
	}
	second()
	select {
	case <-ctx.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("tracker did not stop after the last tab disconnected")
	}
}
