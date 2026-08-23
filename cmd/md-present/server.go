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

//go:embed web/app.js web/index.html web/mermaid.LICENSE.txt web/mermaid.min.js web/style.css
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
}

type presentationState struct {
	mu          sync.RWMutex
	slides      []template.HTML
	revision    uint64
	subscribers map[chan uint64]struct{}
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

func presentationHandler(presentation *presentationState, tracker *tabTracker, diagnostics io.Writer) http.Handler {
	assets, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	assetHandler := http.StripPrefix("/assets/", http.FileServer(http.FS(assets)))
	overflowWarnings := newOverflowReporter(diagnostics)
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", assetHandler)
	mux.HandleFunc("POST /api/overflow", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "http://"+r.Host {
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
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		slides, revision := presentation.snapshot()
		if err := pageTemplate.Execute(w, pageData{Slides: slides, Revision: revision}); err != nil {
			http.Error(w, "render presentation", http.StatusInternalServerError)
		}
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https: http:; media-src 'self' data: https: http:; style-src 'self'; script-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}

func servePresentation(markdownPath string, slides []template.HTML, noOpen bool, stdout, stderr io.Writer) error {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start local server: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	tracker := newTabTracker(stop, tabCloseGrace)
	presentation := newPresentationState(slides)
	server := &http.Server{
		Handler:           presentationHandler(presentation, tracker, stderr),
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(listener)
	}()
	go watchPresentation(ctx, newPresentationWatcher(markdownPath), presentation, liveReloadPoll, stderr)

	url := "http://" + listener.Addr().String() + "/"
	fmt.Fprintln(stdout, url)
	if !noOpen {
		if err := openBrowser(url); err != nil {
			stop()
			shutdownServer(server)
			<-serverErrors
			return fmt.Errorf("open browser: %w", err)
		}
	}

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve presentation: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownServer(server)
		err := <-serverErrors
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve presentation: %w", err)
		}
		return nil
	}
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

func watchPresentation(ctx context.Context, watcher *presentationWatcher, presentation *presentationState, interval time.Duration, stderr io.Writer) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if watcher.changed() {
				_ = refreshPresentationWithWarnings(watcher.markdownPath, presentation, stderr)
			}
		case <-ctx.Done():
			return
		}
	}
}

func refreshPresentation(markdownPath string, presentation *presentationState) error {
	return refreshPresentationWithWarnings(markdownPath, presentation, nil)
}

func refreshPresentationWithWarnings(markdownPath string, presentation *presentationState, warnings io.Writer) error {
	source, err := os.ReadFile(markdownPath)
	if err != nil {
		return err
	}
	slides, err := renderSlidesWithWarnings(source, filepath.Dir(markdownPath), warnings)
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
