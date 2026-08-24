package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewerVersion(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		current   string
		want      bool
	}{
		{name: "newer patch", candidate: "0.2.8", current: "0.2.7", want: true},
		{name: "newer minor", candidate: "0.3.0", current: "0.2.7", want: true},
		{name: "same version", candidate: "v0.2.7", current: "0.2.7"},
		{name: "older version", candidate: "0.2.6", current: "0.2.7"},
		{name: "pre-release ignored", candidate: "0.2.8-rc.1", current: "0.2.7"},
		{name: "invalid current", candidate: "0.2.8", current: "development"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := newerVersion(test.candidate, test.current); got != test.want {
				t.Fatalf("newerVersion(%q, %q) = %t, want %t", test.candidate, test.current, got, test.want)
			}
		})
	}
}

func TestCheckForUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			http.Error(w, fmt.Sprintf("Accept = %q", got), http.StatusBadRequest)
			return
		}
		if got := r.Header.Get("User-Agent"); got != "md-present/0.2.7" {
			http.Error(w, fmt.Sprintf("User-Agent = %q", got), http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"0.2.8"}`))
	}))
	defer server.Close()

	originalURL := latestReleaseURL
	latestReleaseURL = server.URL
	t.Cleanup(func() { latestReleaseURL = originalURL })

	version, err := checkForUpdate(context.Background(), server.Client(), "0.2.7")
	if err != nil {
		t.Fatal(err)
	}
	if version != "0.2.8" {
		t.Fatalf("update version = %q, want 0.2.8", version)
	}
}
