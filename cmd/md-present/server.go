package main

import (
	"context"
	"embed"
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
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed web/*
var webFiles embed.FS

var pageTemplate = template.Must(template.New("index.html").Funcs(template.FuncMap{
	"inc": func(value int) int { return value + 1 },
}).ParseFS(webFiles, "web/index.html"))

const (
	shutdownTimeout = 3 * time.Second
	tabCloseGrace   = 2 * time.Second
)

type pageData struct {
	Slides []template.HTML
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

func presentationHandler(slides []template.HTML, tracker *tabTracker) http.Handler {
	assets, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	assetHandler := http.StripPrefix("/assets/", http.FileServer(http.FS(assets)))
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", assetHandler)
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "connected\n")
		flusher.Flush()
		disconnected := tracker.connected()
		defer disconnected()
		<-r.Context().Done()
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := pageTemplate.Execute(w, pageData{Slides: slides}); err != nil {
			http.Error(w, "render presentation", http.StatusInternalServerError)
		}
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https: http:; style-src 'self'; script-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}

func servePresentation(slides []template.HTML, noOpen bool, stdout io.Writer) error {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start local server: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	tracker := newTabTracker(stop, tabCloseGrace)
	server := &http.Server{
		Handler:           presentationHandler(slides, tracker),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(listener)
	}()

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
