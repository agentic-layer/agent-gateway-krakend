package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
)

// TestPluginRegistration verifies the plugin can be registered
func TestPluginRegistration(t *testing.T) {
	if HandlerRegisterer == "" {
		t.Error("HandlerRegisterer is empty")
	}

	var called bool
	HandlerRegisterer.RegisterHandlers(func(name string, handler func(context.Context, map[string]interface{}, http.Handler) (http.Handler, error)) {
		called = true
		if name != pluginName {
			t.Errorf("RegisterHandlers called with name %q, want %q", name, pluginName)
		}
	})

	if !called {
		t.Error("RegisterHandlers function was not called")
	}
}

// TestRequestPassThrough verifies non-agent-card requests pass through unchanged
func TestRequestPassThrough(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "POST request",
			method: http.MethodPost,
			path:   "/weather-agent/.well-known/agent-card.json",
		},
		{
			name:   "GET to non-agent-card endpoint",
			method: http.MethodGet,
			path:   "/api/agents",
		},
		{
			name:   "GET to chat completions",
			method: http.MethodGet,
			path:   "/weather-agent/chat/completions",
		},
		{
			name:   "GET to root",
			method: http.MethodGet,
			path:   "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock backend that returns a simple response
			backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("backend response"))
			})

			// Create the plugin handler
			handler, err := HandlerRegisterer.registerHandlers(context.Background(), map[string]interface{}{}, backend)
			if err != nil {
				t.Fatalf("failed to register handler: %v", err)
			}

			// Create test request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Host = "gateway.agentic-layer.ai"
			req.Header.Set("X-Forwarded-Proto", "https")

			// Record response
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// Verify response passed through unchanged
			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if rec.Body.String() != "backend response" {
				t.Errorf("body = %q, want %q", rec.Body.String(), "backend response")
			}
		})
	}
}

// TestAgentCardInterception verifies agent card requests are intercepted
func TestAgentCardInterception(t *testing.T) {
	// Create a mock backend that returns an agent card with internal URLs
	agentCard := models.AgentCard{
		Name:        "Test Agent",
		Description: "A test agent",
		Url:         "http://test-agent.default.svc.cluster.local:8000/",
		Version:     "1.0.0",
		AdditionalInterfaces: []models.AgentInterface{
			{Transport: "http", Url: "http://test-agent.default.svc.cluster.local:8000/"},
			{Transport: "grpc", Url: "http://test-agent.default.svc.cluster.local:9000/"},
		},
		Provider: &models.AgentProvider{
			Organization: "Test Org",
			Url:          "https://qaware.de",
		},
	}

	cardJSON, _ := json.Marshal(agentCard)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "successful-transform-request-id")
		w.Header().Set("X-Custom-Header", "backend-custom-value")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.WriteHeader(http.StatusOK)
		w.Write(cardJSON)
	})

	// Create the plugin handler
	handler, err := HandlerRegisterer.registerHandlers(context.Background(), map[string]interface{}{}, backend)
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/test-agent/.well-known/agent-card.json", nil)
	req.Host = "gateway.agentic-layer.ai"
	req.Header.Set("X-Forwarded-Proto", "https")

	// Record response
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify response was transformed
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var responseCard models.AgentCard
	if err := json.Unmarshal(rec.Body.Bytes(), &responseCard); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Todo clarify external Gateway URL construction
	// Verify URL was rewritten
	expectedURL := "https://gateway.agentic-layer.ai/test-agent"
	if responseCard.Url != expectedURL {
		t.Errorf("card.Url = %q, want %q", responseCard.Url, expectedURL)
	}

	// Verify grpc interface was removed
	if len(responseCard.AdditionalInterfaces) != 1 {
		t.Errorf("len(AdditionalInterfaces) = %d, want 1", len(responseCard.AdditionalInterfaces))
	}

	// Verify http interface was rewritten
	if len(responseCard.AdditionalInterfaces) > 0 {
		if responseCard.AdditionalInterfaces[0].Url != expectedURL {
			t.Errorf("AdditionalInterfaces[0].Url = %q, want %q",
				responseCard.AdditionalInterfaces[0].Url, expectedURL)
		}
	}

	// Verify provider URL was not changed
	if responseCard.Provider.Url != "https://qaware.de" {
		t.Errorf("Provider.Url = %q, want unchanged", responseCard.Provider.Url)
	}

	// Verify backend headers are preserved during transformation
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type header = %q, want %q", rec.Header().Get("Content-Type"), "application/json")
	}
	if rec.Header().Get("X-Request-ID") != "successful-transform-request-id" {
		t.Errorf("X-Request-ID header = %q, want %q", rec.Header().Get("X-Request-ID"), "successful-transform-request-id")
	}
	if rec.Header().Get("X-Custom-Header") != "backend-custom-value" {
		t.Errorf("X-Custom-Header = %q, want %q", rec.Header().Get("X-Custom-Header"), "backend-custom-value")
	}
	if rec.Header().Get("Cache-Control") != "max-age=3600" {
		t.Errorf("Cache-Control = %q, want %q", rec.Header().Get("Cache-Control"), "max-age=3600")
	}
}

// TestNonOKStatusPassThrough verifies non-OK responses pass through unchanged
func TestNonOKStatusPassThrough(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "404 Not Found",
			statusCode: http.StatusNotFound,
			body:       "not found",
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			body:       "internal error",
		},
		{
			name:       "403 Forbidden",
			statusCode: http.StatusForbidden,
			body:       "forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock backend that returns non-OK status
			backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Request-ID", "test-request-id-123")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("X-Custom-Header", "custom-value")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			})

			// Create the plugin handler
			handler, err := HandlerRegisterer.registerHandlers(context.Background(), map[string]interface{}{}, backend)
			if err != nil {
				t.Fatalf("failed to register handler: %v", err)
			}

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/test-agent/.well-known/agent-card.json", nil)
			req.Host = "gateway.agentic-layer.ai"
			req.Header.Set("X-Forwarded-Proto", "https")

			// Record response
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// Verify response passed through unchanged
			if rec.Code != tt.statusCode {
				t.Errorf("status code = %d, want %d", rec.Code, tt.statusCode)
			}
			if rec.Body.String() != tt.body {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.body)
			}

			// Verify headers are preserved
			if rec.Header().Get("X-Request-ID") != "test-request-id-123" {
				t.Errorf("X-Request-ID header = %q, want %q", rec.Header().Get("X-Request-ID"), "test-request-id-123")
			}
			if rec.Header().Get("Cache-Control") != "no-cache" {
				t.Errorf("Cache-Control header = %q, want %q", rec.Header().Get("Cache-Control"), "no-cache")
			}
			if rec.Header().Get("X-Custom-Header") != "custom-value" {
				t.Errorf("X-Custom-Header = %q, want %q", rec.Header().Get("X-Custom-Header"), "custom-value")
			}
		})
	}
}

// TestMalformedJSONPassThrough verifies malformed JSON responses pass through
func TestMalformedJSONPassThrough(t *testing.T) {
	malformedJSON := `{"invalid": json}`

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "malformed-json-request-id")
		w.Header().Set("X-Custom-Header", "malformed-json-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(malformedJSON))
	})

	// Create the plugin handler
	handler, err := HandlerRegisterer.registerHandlers(context.Background(), map[string]interface{}{}, backend)
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/test-agent/.well-known/agent-card.json", nil)
	req.Host = "gateway.agentic-layer.ai"
	req.Header.Set("X-Forwarded-Proto", "https")

	// Record response
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify malformed JSON passed through unchanged
	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != malformedJSON {
		t.Errorf("body = %q, want %q", rec.Body.String(), malformedJSON)
	}

	// Verify headers are preserved
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type header = %q, want %q", rec.Header().Get("Content-Type"), "application/json")
	}
	if rec.Header().Get("X-Request-ID") != "malformed-json-request-id" {
		t.Errorf("X-Request-ID header = %q, want %q", rec.Header().Get("X-Request-ID"), "malformed-json-request-id")
	}
	if rec.Header().Get("X-Custom-Header") != "malformed-json-value" {
		t.Errorf("X-Custom-Header = %q, want %q", rec.Header().Get("X-Custom-Header"), "malformed-json-value")
	}
}

// TestNonJSONContentTypePassThrough verifies non-JSON responses pass through
func TestNonJSONContentTypePassThrough(t *testing.T) {
	htmlResponse := "<html><body>Not JSON</body></html>"

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("X-Request-ID", "html-request-id")
		w.Header().Set("X-Custom-Header", "html-custom-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(htmlResponse))
	})

	// Create the plugin handler
	handler, err := HandlerRegisterer.registerHandlers(context.Background(), map[string]interface{}{}, backend)
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/test-agent/.well-known/agent-card.json", nil)
	req.Host = "gateway.agentic-layer.ai"
	req.Header.Set("X-Forwarded-Proto", "https")

	// Record response
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify HTML response passed through unchanged
	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != htmlResponse {
		t.Errorf("body = %q, want %q", rec.Body.String(), htmlResponse)
	}

	// Verify headers are preserved
	if rec.Header().Get("Content-Type") != "text/html" {
		t.Errorf("Content-Type header = %q, want %q", rec.Header().Get("Content-Type"), "text/html")
	}
	if rec.Header().Get("X-Request-ID") != "html-request-id" {
		t.Errorf("X-Request-ID header = %q, want %q", rec.Header().Get("X-Request-ID"), "html-request-id")
	}
	if rec.Header().Get("X-Custom-Header") != "html-custom-value" {
		t.Errorf("X-Custom-Header = %q, want %q", rec.Header().Get("X-Custom-Header"), "html-custom-value")
	}
}

// TestInternalRequestPassThrough verifies internal cluster requests skip rewriting
func TestInternalRequestPassThrough(t *testing.T) {
	agentCard := models.AgentCard{
		Name: "Test Agent",
		Url:  "http://test-agent.default.svc.cluster.local:8000/",
	}
	cardJSON, _ := json.Marshal(agentCard)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(cardJSON)
	})

	// Create the plugin handler
	handler, err := HandlerRegisterer.registerHandlers(context.Background(), map[string]interface{}{}, backend)
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// Create test request with internal cluster host
	req := httptest.NewRequest(http.MethodGet, "/test-agent/.well-known/agent-card.json", nil)
	req.Host = "agent-gateway.default.svc.cluster.local:10000"
	req.Header.Set("X-Forwarded-Proto", "http")

	// Record response
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify response passed through unchanged (no rewriting for internal requests)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var responseCard models.AgentCard
	if err := json.Unmarshal(rec.Body.Bytes(), &responseCard); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// URL should remain internal (not rewritten)
	if responseCard.Url != agentCard.Url {
		t.Errorf("card.Url = %q, want unchanged %q", responseCard.Url, agentCard.Url)
	}
}

// TestExternalURLPreserved verifies external URLs in agent cards are not rewritten
func TestExternalURLPreserved(t *testing.T) {
	externalURL := "https://external-agent.example.com/api"
	agentCard := models.AgentCard{
		Name: "External Agent",
		Url:  externalURL,
	}
	cardJSON, _ := json.Marshal(agentCard)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(cardJSON)
	})

	// Create the plugin handler
	handler, err := HandlerRegisterer.registerHandlers(context.Background(), map[string]interface{}{}, backend)
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/external-agent/.well-known/agent-card.json", nil)
	req.Host = "gateway.agentic-layer.ai"
	req.Header.Set("X-Forwarded-Proto", "https")

	// Record response
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var responseCard models.AgentCard
	if err := json.Unmarshal(rec.Body.Bytes(), &responseCard); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// External URL should be preserved
	if responseCard.Url != externalURL {
		t.Errorf("card.Url = %q, want preserved %q", responseCard.Url, externalURL)
	}
}

// TestResponseWriterCapture verifies the responseWriter correctly captures responses
func TestResponseWriterCapture(t *testing.T) {
	originalWriter := httptest.NewRecorder()
	rw := newResponseWriter(originalWriter)

	// Write status and body
	rw.WriteHeader(http.StatusCreated)
	rw.Write([]byte("test body"))

	// Verify captured status
	if rw.statusCode != http.StatusCreated {
		t.Errorf("captured statusCode = %d, want %d", rw.statusCode, http.StatusCreated)
	}

	// Verify captured body
	if rw.body.String() != "test body" {
		t.Errorf("captured body = %q, want %q", rw.body.String(), "test body")
	}
}

// TestEmptyAgentNamePassThrough verifies requests without agent names pass through
func TestEmptyAgentNamePassThrough(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend response"))
	})

	// Create the plugin handler
	handler, err := HandlerRegisterer.registerHandlers(context.Background(), map[string]interface{}{}, backend)
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// Create test request to root well-known (no agent name)
	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	req.Host = "gateway.agentic-layer.ai"
	req.Header.Set("X-Forwarded-Proto", "https")

	// Record response
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify response passed through unchanged
	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "backend response" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "backend response")
	}
}
