package main

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPresentationHandler(t *testing.T) {
	tracker := newTabTracker(func() {}, time.Second)
	handler := presentationHandler([]template.HTML{"<h1>Expected slide</h1>"}, tracker)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Expected slide") {
		t.Fatalf("GET / status %d, body:\n%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("GET / omitted Content-Security-Policy")
	}

	assetRequest := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	assetResponse := httptest.NewRecorder()
	handler.ServeHTTP(assetResponse, assetRequest)
	if assetResponse.Code != http.StatusOK {
		t.Fatalf("GET /assets/app.js status = %d", assetResponse.Code)
	}

	missingRequest := httptest.NewRequest(http.MethodGet, "/source.md", nil)
	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, missingRequest)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("GET /source.md status = %d, want 404", missingResponse.Code)
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
