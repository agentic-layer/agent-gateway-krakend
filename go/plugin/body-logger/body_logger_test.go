package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newHandler(t *testing.T, extra map[string]interface{}, next http.Handler) http.Handler {
	t.Helper()
	h, err := HandlerRegisterer.registerHandlers(context.Background(), extra, next)
	if err != nil {
		t.Fatalf("registerHandlers failed: %v", err)
	}
	return h
}

// TestHandleRequest_LogsRequestAndResponse verifies that a normal request with a body
// is passed through to the next handler and the response body/status are forwarded correctly.
func TestHandleRequest_LogsRequestAndResponse(t *testing.T) {
	const responseBody = `{"result":"ok"}`
	const requestBody = `{"hello":"world"}`

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(responseBody))
	})

	handler := newHandler(t, map[string]interface{}{}, backend)

	req := httptest.NewRequest(http.MethodPost, "/some/path", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if rec.Body.String() != responseBody {
		t.Errorf("body = %q, want %q", rec.Body.String(), responseBody)
	}
}

// TestHandleRequest_SkipPaths verifies that requests to a configured skip_path
// bypass the captureWriter and are written directly to the real ResponseWriter.
func TestHandleRequest_SkipPaths(t *testing.T) {
	const responseBody = "health ok"

	var backendWriter http.ResponseWriter
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendWriter = w
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseBody))
	})

	extra := map[string]interface{}{
		"body_logger_config": map[string]interface{}{
			"skip_paths": []interface{}{"/__health"},
		},
	}

	handler := newHandler(t, extra, backend)

	req := httptest.NewRequest(http.MethodGet, "/__health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != responseBody {
		t.Errorf("body = %q, want %q", rec.Body.String(), responseBody)
	}

	// For a skipped path the handler should write directly to the real ResponseWriter,
	// not a captureWriter – verify the writer seen by the backend is not a captureWriter.
	if _, isCaptured := backendWriter.(*captureWriter); isCaptured {
		t.Error("skipped path should not wrap ResponseWriter in captureWriter")
	}
}

// TestHandleRequest_NoSkipPaths verifies that when no skip_paths are configured all
// requests are processed (body captured and forwarded) without error.
func TestHandleRequest_NoSkipPaths(t *testing.T) {
	const responseBody = "pong"

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseBody))
	})

	// No body_logger_config at all – no skip_paths.
	handler := newHandler(t, map[string]interface{}{}, backend)

	for _, path := range []string{"/", "/api/v1/agents", "/__health"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("path %s: status = %d, want %d", path, rec.Code, http.StatusOK)
			}
			if rec.Body.String() != responseBody {
				t.Errorf("path %s: body = %q, want %q", path, rec.Body.String(), responseBody)
			}
		})
	}
}

// TestParseConfig_ValidConfig verifies that a well-formed body_logger_config is parsed correctly.
func TestParseConfig_ValidConfig(t *testing.T) {
	extra := map[string]interface{}{
		"body_logger_config": map[string]interface{}{
			"skip_paths": []interface{}{"/__health", "/metrics"},
		},
	}

	cfg, err := parseConfig(extra)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.SkipPaths) != 2 {
		t.Fatalf("len(SkipPaths) = %d, want 2", len(cfg.SkipPaths))
	}
	if cfg.SkipPaths[0] != "/__health" {
		t.Errorf("SkipPaths[0] = %q, want %q", cfg.SkipPaths[0], "/__health")
	}
	if cfg.SkipPaths[1] != "/metrics" {
		t.Errorf("SkipPaths[1] = %q, want %q", cfg.SkipPaths[1], "/metrics")
	}
}

// TestParseConfig_MissingConfig verifies that an absent body_logger_config key returns
// an empty config without error.
func TestParseConfig_MissingConfig(t *testing.T) {
	cfg, err := parseConfig(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.SkipPaths) != 0 {
		t.Errorf("SkipPaths = %v, want empty", cfg.SkipPaths)
	}
}

// TestParseConfig_InvalidConfig verifies that a malformed config value (one that cannot be
// marshalled into the config struct) returns an error.
func TestParseConfig_InvalidConfig(t *testing.T) {
	// A channel cannot be JSON-marshalled, so json.Marshal will fail.
	extra := map[string]interface{}{
		"body_logger_config": make(chan int),
	}

	_, err := parseConfig(extra)
	if err == nil {
		t.Error("expected an error for invalid config, got nil")
	}
}
