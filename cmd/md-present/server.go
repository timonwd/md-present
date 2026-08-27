package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web/app.js web/favicon.svg web/index.html web/mermaid.LICENSE.txt web/mermaid.min.js web/style.css
var webFiles embed.FS

var pageTemplate = template.Must(template.New("index.html").Funcs(template.FuncMap{
	"inc": func(value int) int { return value + 1 },
}).ParseFS(webFiles, "web/index.html"))

const (
	shutdownTimeout = 3 * time.Second
	tabCloseGrace   = 2 * time.Second
	liveReloadPoll  = 250 * time.Millisecond
)

type pageData struct {
	Slides   []template.HTML
	Revision uint64
	Title    string
	Editor   bool
}

type presentationState struct {
	mu          sync.RWMutex
	slides      []template.HTML
	revision    uint64
	subscribers map[chan uint64]struct{}
}

const maxEditorSourceBytes = 1 << 20

type editorSaveRequest struct {
	Source   string `json:"source"`
	Revision string `json:"revision"`
	Slide    int    `json:"slide"`
}

type editorSourceResponse struct {
	Source   string `json:"source"`
	Revision string `json:"revision"`
	Slide    int    `json:"slide"`
	Slides   int    `json:"slides"`
}

type editorSaveResponse struct {
	Revision     string `json:"revision"`
	DeckRevision uint64 `json:"deckRevision"`
	HTML         string `json:"html"`
}

type editorSource struct {
	path         string
	presentation *presentationState
	options      renderOptions
	mu           sync.Mutex
}

type presentationWatcher struct {
	markdownPath string
	fingerprint  [sha256.Size]byte
}

type overflowReport struct {
	Revision    uint64 `json:"revision"`
	Slides      []int  `json:"slides"`
	StageWidth  int    `json:"stageWidth"`
	StageHeight int    `json:"stageHeight"`
}

type overflowReporter struct {
	mu       sync.Mutex
	writer   io.Writer
	revision uint64
	seen     map[string]struct{}
}

type runningPresentation struct {
	url  string
	done <-chan error
}

func (p *runningPresentation) wait() error {
	return <-p.done
}

func newOverflowReporter(writer io.Writer) *overflowReporter {
	return &overflowReporter{writer: writer, seen: make(map[string]struct{})}
}

func (r *overflowReporter) report(report overflowReport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if report.Revision != r.revision {
		r.revision = report.Revision
		clear(r.seen)
	}
	fingerprint := fmt.Sprint(report.Slides)
	if _, reported := r.seen[fingerprint]; reported {
		return
	}
	r.seen[fingerprint] = struct{}{}

	slideLabel := "slides"
	verb := "exceed"
	if len(report.Slides) == 1 {
		slideLabel = "slide"
		verb = "exceeds"
	}
	fmt.Fprintf(
		r.writer,
		"md-present: warning: %s %s %s the regular 16:9 slide area at %dx%d; scroll to view all content\n",
		slideLabel,
		formatSlideNumbers(report.Slides),
		verb,
		report.StageWidth,
		report.StageHeight,
	)
}

func formatSlideNumbers(slides []int) string {
	values := make([]string, len(slides))
	for index, slide := range slides {
		values[index] = strconv.Itoa(slide)
	}
	return strings.Join(values, ", ")
}

func newPresentationState(slides []template.HTML) *presentationState {
	return &presentationState{
		slides:      slides,
		revision:    1,
		subscribers: make(map[chan uint64]struct{}),
	}
}

func (p *presentationState) snapshot() ([]template.HTML, uint64) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.slides, p.revision
}

func (p *presentationState) update(slides []template.HTML) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if slices.Equal(p.slides, slides) {
		return false
	}
	p.slides = slides
	p.revision++
	for subscriber := range p.subscribers {
		select {
		case subscriber <- p.revision:
		default:
		}
	}
	return true
}

func (p *presentationState) subscribe() (<-chan uint64, uint64, func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	updates := make(chan uint64, 1)
	p.subscribers[updates] = struct{}{}
	return updates, p.revision, func() {
		p.mu.Lock()
		delete(p.subscribers, updates)
		p.mu.Unlock()
	}
}

type tabTracker struct {
	mu       sync.Mutex
	active   int
	shutdown context.CancelFunc
	timer    *time.Timer
	grace    time.Duration
}

func newTabTracker(shutdown context.CancelFunc, grace time.Duration) *tabTracker {
	return &tabTracker{shutdown: shutdown, grace: grace}
}

func (t *tabTracker) connected() func() {
	t.mu.Lock()
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
	t.active++
	t.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			t.mu.Lock()
			t.active--
			if t.active == 0 {
				t.timer = time.AfterFunc(t.grace, t.shutdown)
			}
			t.mu.Unlock()
		})
	}
}

func presentationHandler(presentation *presentationState, tracker *tabTracker, diagnostics io.Writer, titles ...string) http.Handler {
	return presentationHandlerWithUpdate(presentation, tracker, diagnostics, nil, titles...)
}

func presentationHandlerWithUpdate(presentation *presentationState, tracker *tabTracker, diagnostics io.Writer, updates *updateState, titles ...string) http.Handler {
	return presentationHandlerWithOptions(presentation, tracker, diagnostics, updates, renderOptions{}, titles...)
}

func presentationHandlerWithOptions(presentation *presentationState, tracker *tabTracker, diagnostics io.Writer, updates *updateState, options renderOptions, titles ...string) http.Handler {
	if updates == nil {
		updates = newUpdateState()
		updates.set("")
	}
	title := "md-present"
	if len(titles) > 0 && titles[0] != "" {
		title = titles[0]
	}
	assets, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	assetHandler := http.StripPrefix("/assets/", http.FileServer(http.FS(assets)))
	overflowWarnings := newOverflowReporter(diagnostics)
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", assetHandler)
	if options.markdownPath != "" {
		editor := &editorSource{path: options.markdownPath, presentation: presentation, options: options}
		mux.HandleFunc("GET /api/source", func(w http.ResponseWriter, r *http.Request) {
			slide, err := editorSlide(r.URL.Query().Get("slide"))
			if err != nil {
				http.Error(w, "invalid slide", http.StatusBadRequest)
				return
			}
			source, revision, slides, err := editor.read(slide)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(editorSourceResponse{Source: source, Revision: revision, Slide: slide, Slides: slides})
		})
		mux.HandleFunc("PUT /api/source", func(w http.ResponseWriter, r *http.Request) {
			if !sameOrigin(r) {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
			var request editorSaveRequest
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxEditorSourceBytes+1024))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil || decoder.Decode(&struct{}{}) != io.EOF || len(request.Source) > maxEditorSourceBytes || request.Revision == "" || request.Slide < 1 {
				http.Error(w, "invalid presentation source", http.StatusBadRequest)
				return
			}
			saved, err := editor.save(request)
			if errors.Is(err, errEditorConflict) {
				http.Error(w, "presentation source changed outside this editor", http.StatusConflict)
				return
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(saved)
		})
		mux.HandleFunc("POST /api/source/preview", func(w http.ResponseWriter, r *http.Request) {
			if !sameOrigin(r) {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
			var request editorSaveRequest
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxEditorSourceBytes+1024))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil || decoder.Decode(&struct{}{}) != io.EOF || len(request.Source) > maxEditorSourceBytes || request.Revision == "" || request.Slide < 1 {
				http.Error(w, "invalid presentation source", http.StatusBadRequest)
				return
			}
			preview, err := editor.preview(request)
			if errors.Is(err, errEditorConflict) {
				http.Error(w, "presentation source changed outside this editor", http.StatusConflict)
				return
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(preview)
		})
	}
	mux.HandleFunc("POST /api/overflow", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			http.Error(w, "cross-origin request rejected", http.StatusForbidden)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
			return
		}

		var report overflowReport
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&report); err != nil {
			http.Error(w, "invalid overflow report", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "invalid overflow report", http.StatusBadRequest)
			return
		}
		slides, revision := presentation.snapshot()
		if report.Revision != revision {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if report.StageWidth <= 0 || report.StageHeight <= 0 || report.StageWidth > 100_000 || report.StageHeight > 100_000 || len(report.Slides) == 0 || len(report.Slides) > len(slides) {
			http.Error(w, "invalid overflow report", http.StatusBadRequest)
			return
		}
		previous := 0
		for _, slide := range report.Slides {
			if slide <= previous || slide > len(slides) {
				http.Error(w, "invalid overflow report", http.StatusBadRequest)
				return
			}
			previous = slide
		}

		overflowWarnings.report(report)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		updates, revision, unsubscribe := presentation.subscribe()
		defer unsubscribe()
		_, _ = fmt.Fprintf(w, "event: ready\ndata: %d\n\n", revision)
		flusher.Flush()
		disconnected := tracker.connected()
		defer disconnected()

		clientRevision, _ := strconv.ParseUint(r.URL.Query().Get("revision"), 10, 64)
		if clientRevision != revision {
			_, _ = fmt.Fprintf(w, "event: reload\ndata: %d\n\n", revision)
			flusher.Flush()
		}

		for {
			select {
			case revision := <-updates:
				if _, err := fmt.Fprintf(w, "event: reload\ndata: %d\n\n", revision); err != nil {
					return
				}
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
	mux.HandleFunc("GET /api/update", func(w http.ResponseWriter, _ *http.Request) {
		status, done := updates.snapshot()
		if !done {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			return
		}
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		slides, revision := presentation.snapshot()
		if err := pageTemplate.Execute(w, pageData{Slides: slides, Revision: revision, Title: title, Editor: options.markdownPath != ""}); err != nil {
			http.Error(w, "render presentation", http.StatusInternalServerError)
		}
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedPresentationHost(r.Host) {
			http.Error(w, "invalid Host header", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https: http:; media-src 'self' data: https: http:; style-src 'self'; script-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}

var errEditorConflict = errors.New("presentation source changed")

func editorSlide(value string) (int, error) {
	slide, err := strconv.Atoi(value)
	if err != nil || slide < 1 {
		return 0, errors.New("invalid slide")
	}
	return slide, nil
}

func (e *editorSource) read(slide int) (string, string, int, error) {
	source, revision, err := e.readDeck()
	if err != nil {
		return "", "", 0, err
	}
	segments := slideSegments(source)
	if slide > len(segments) {
		return "", "", 0, fmt.Errorf("slide %d does not exist", slide)
	}
	segment := segments[slide-1]
	return source[segment.start:segment.stop], revision, len(segments), nil
}

func (e *editorSource) readDeck() (string, string, error) {
	source, err := os.ReadFile(e.path)
	if err != nil || len(source) > maxEditorSourceBytes {
		if err == nil {
			err = fmt.Errorf("presentation source exceeds %d bytes", maxEditorSourceBytes)
		}
		return "", "", err
	}
	return string(source), sourceRevision(source), nil
}

func (e *editorSource) save(request editorSaveRequest) (editorSaveResponse, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	current, revision, err := e.readDeck()
	if err != nil {
		return editorSaveResponse{}, err
	}
	if revision != request.Revision {
		return editorSaveResponse{}, errEditorConflict
	}
	segments := slideSegments(current)
	if request.Slide > len(segments) {
		return editorSaveResponse{}, fmt.Errorf("slide %d does not exist", request.Slide)
	}
	segment := segments[request.Slide-1]
	candidate := current[:segment.start] + request.Source + current[segment.stop:]
	if candidate == current {
		slides, deckRevision := e.presentation.snapshot()
		return editorSaveResponse{Revision: revision, DeckRevision: deckRevision, HTML: string(slides[request.Slide-1])}, nil
	}
	slides, err := renderSlidesWithOptions([]byte(candidate), filepath.Dir(e.path), nil, e.options)
	if err != nil {
		return editorSaveResponse{}, err
	}
	if len(slides) != len(segments) {
		return editorSaveResponse{}, errors.New("editing slide separators is not supported")
	}
	info, err := os.Stat(e.path)
	if err != nil {
		return editorSaveResponse{}, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(e.path), ".md-present-*")
	if err != nil {
		return editorSaveResponse{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(info.Mode().Perm()); err == nil {
		_, err = temporary.WriteString(candidate)
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return editorSaveResponse{}, err
	}
	if err := os.Rename(temporaryPath, e.path); err != nil {
		return editorSaveResponse{}, err
	}
	e.presentation.update(slides)
	_, deckRevision := e.presentation.snapshot()
	return editorSaveResponse{Revision: sourceRevision([]byte(candidate)), DeckRevision: deckRevision, HTML: string(slides[request.Slide-1])}, nil
}

func (e *editorSource) preview(request editorSaveRequest) (editorSaveResponse, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	current, revision, err := e.readDeck()
	if err != nil {
		return editorSaveResponse{}, err
	}
	if revision != request.Revision {
		return editorSaveResponse{}, errEditorConflict
	}
	segments := slideSegments(current)
	if request.Slide > len(segments) {
		return editorSaveResponse{}, fmt.Errorf("slide %d does not exist", request.Slide)
	}
	segment := segments[request.Slide-1]
	candidate := current[:segment.start] + request.Source + current[segment.stop:]
	slides, err := renderSlidesWithOptions([]byte(candidate), filepath.Dir(e.path), nil, e.options)
	if err != nil {
		return editorSaveResponse{}, err
	}
	if len(slides) != len(segments) {
		return editorSaveResponse{}, errors.New("editing slide separators is not supported")
	}
	_, deckRevision := e.presentation.snapshot()
	return editorSaveResponse{Revision: revision, DeckRevision: deckRevision, HTML: string(slides[request.Slide-1])}, nil
}

func sourceRevision(source []byte) string { return fmt.Sprintf("%x", sha256.Sum256(source)) }

func sameOrigin(r *http.Request) bool { return r.Header.Get("Origin") == "http://"+r.Host }

func servePresentation(markdownPath string, slides []template.HTML, noOpen bool, options renderOptions, stdout, stderr io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	presentation, err := startPresentation(ctx, markdownPath, slides, options, !noOpen, stdout, stderr)
	if err != nil {
		return err
	}
	return presentation.wait()
}

func startPresentation(parent context.Context, markdownPath string, slides []template.HTML, options renderOptions, open bool, stdout, stderr io.Writer) (*runningPresentation, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start local server: %w", err)
	}

	ctx, stop := context.WithCancel(parent)
	tracker := newTabTracker(stop, tabCloseGrace)
	presentation := newPresentationState(slides)
	updates := newUpdateState()
	server := &http.Server{
		Handler:           presentationHandlerWithOptions(presentation, tracker, stderr, updates, withMarkdownPath(options, markdownPath), presentationTitle(markdownPath)),
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(listener)
	}()
	go watchPresentation(ctx, newPresentationWatcher(markdownPath), presentation, liveReloadPoll, options, stderr)
	go func() {
		checkContext, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		availableVersion, err := checkForUpdate(checkContext, http.DefaultClient, version)
		if err == nil && availableVersion != "" {
			fmt.Fprintf(stderr, "md-present: version %s is available; run brew upgrade --cask md-present to update\n", availableVersion)
		}
		updates.set(availableVersion)
	}()

	url := "http://" + listener.Addr().String() + "/"
	if stdout != nil {
		fmt.Fprintln(stdout, url)
	}
	if open {
		if err := openBrowser(url); err != nil {
			stop()
			shutdownServer(server)
			<-serverErrors
			return nil, fmt.Errorf("open browser: %w", err)
		}
	}

	done := make(chan error, 1)
	go func() {
		defer stop()
		select {
		case err := <-serverErrors:
			if !errors.Is(err, http.ErrServerClosed) {
				done <- fmt.Errorf("serve presentation: %w", err)
				return
			}
			done <- nil
		case <-ctx.Done():
			shutdownServer(server)
			err := <-serverErrors
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				done <- fmt.Errorf("serve presentation: %w", err)
				return
			}
			done <- nil
		}
	}()

	return &runningPresentation{url: url, done: done}, nil
}

func withMarkdownPath(options renderOptions, path string) renderOptions {
	options.markdownPath = path
	return options
}

func allowedPresentationHost(value string) bool {
	host, _, err := net.SplitHostPort(value)
	return err == nil && (host == "127.0.0.1" || host == "localhost")
}

func presentationTitle(markdownPath string) string {
	return filepath.Base(markdownPath)
}

func newPresentationWatcher(markdownPath string) *presentationWatcher {
	watcher := &presentationWatcher{markdownPath: markdownPath}
	if fingerprint, err := presentationInputFingerprint(markdownPath); err == nil {
		watcher.fingerprint = fingerprint
	}
	return watcher
}

func (w *presentationWatcher) changed() bool {
	fingerprint, err := presentationInputFingerprint(w.markdownPath)
	if err != nil || fingerprint == w.fingerprint {
		return false
	}
	w.fingerprint = fingerprint
	return true
}

func presentationInputFingerprint(markdownPath string) ([sha256.Size]byte, error) {
	source, err := os.ReadFile(markdownPath)
	if err != nil {
		return [sha256.Size]byte{}, err
	}

	hash := sha256.New()
	_, _ = hash.Write(source)
	for _, path := range localMediaPaths(source, filepath.Dir(markdownPath)) {
		_, _ = fmt.Fprintf(hash, "\x00%s\x00", path)
		info, err := os.Stat(path)
		if err != nil {
			_, _ = fmt.Fprintf(hash, "unavailable:%v", err)
			continue
		}
		_, _ = fmt.Fprintf(hash, "%d:%d", info.Size(), info.ModTime().UnixNano())
	}
	var fingerprint [sha256.Size]byte
	copy(fingerprint[:], hash.Sum(nil))
	return fingerprint, nil
}

func watchPresentation(ctx context.Context, watcher *presentationWatcher, presentation *presentationState, interval time.Duration, options renderOptions, stderr io.Writer) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if watcher.changed() {
				if err := refreshPresentationWithOptions(watcher.markdownPath, presentation, stderr, options); err != nil {
					fmt.Fprintf(stderr, "md-present: refresh presentation: %v\n", err)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func refreshPresentation(markdownPath string, presentation *presentationState) error {
	return refreshPresentationWithOptions(markdownPath, presentation, nil, renderOptions{})
}

func refreshPresentationWithWarnings(markdownPath string, presentation *presentationState, warnings io.Writer) error {
	return refreshPresentationWithOptions(markdownPath, presentation, warnings, renderOptions{})
}

func refreshPresentationWithOptions(markdownPath string, presentation *presentationState, warnings io.Writer, options renderOptions) error {
	source, err := os.ReadFile(markdownPath)
	if err != nil {
		return err
	}
	slides, err := renderSlidesWithOptions(source, filepath.Dir(markdownPath), warnings, options)
	if err != nil {
		return err
	}
	presentation.update(slides)
	return nil
}

func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func openBrowser(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "linux":
		command = exec.Command("xdg-open", url)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("automatic browser launch is not supported on %s; use --no-open", runtime.GOOS)
	}
	if output, err := command.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("%w: %s", err, message)
		}
		return err
	}
	return nil
}
