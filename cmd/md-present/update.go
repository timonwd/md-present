package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

var latestReleaseURL = "https://api.github.com/repos/timonwd/md-present/releases/latest"

type releaseResponse struct {
	TagName string `json:"tag_name"`
}

type updateStatus struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
}

type updateState struct {
	mu     sync.RWMutex
	status updateStatus
	done   bool
}

func newUpdateState() *updateState {
	return &updateState{}
}

func (s *updateState) set(version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = updateStatus{Available: version != "", Version: version}
	s.done = true
}

func (s *updateState) snapshot() (updateStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status, s.done
}

func checkForUpdate(ctx context.Context, client *http.Client, currentVersion string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return "", fmt.Errorf("create release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "md-present/"+currentVersion)

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("request latest release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request latest release: unexpected status %s", response.Status)
	}

	var release releaseResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	if err := decoder.Decode(&release); err != nil {
		return "", fmt.Errorf("decode latest release: %w", err)
	}
	if newerVersion(release.TagName, currentVersion) {
		return release.TagName, nil
	}
	return "", nil
}

func newerVersion(candidate, current string) bool {
	candidateParts, ok := parseVersion(candidate)
	if !ok {
		return false
	}
	currentParts, ok := parseVersion(current)
	if !ok {
		return false
	}
	for index := range candidateParts {
		if candidateParts[index] != currentParts[index] {
			return candidateParts[index] > currentParts[index]
		}
	}
	return false
}

func parseVersion(value string) ([3]int, bool) {
	var parts [3]int
	value = strings.TrimPrefix(value, "v")
	segments := strings.Split(value, ".")
	if len(segments) != len(parts) {
		return parts, false
	}
	for index, segment := range segments {
		if segment == "" {
			return parts, false
		}
		for _, character := range segment {
			if character < '0' || character > '9' {
				return parts, false
			}
		}
		for _, character := range segment {
			parts[index] = parts[index]*10 + int(character-'0')
		}
	}
	return parts, true
}
