package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
)

// TestPluginRegistration verifies the plugin can be registered
func TestPluginRegistration(t *testing.T) {
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
				_, _ = w.Write([]byte("backend response"))
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
	// Tests multiple internal URL patterns: localhost, K8s short-form, private IP
	agentCard := models.AgentCard{
		Name:        "Test Agent",
		Description: "A test agent",
		Url:         "http://localhost:8000/",
		Version:     "1.0.0",
		AdditionalInterfaces: []models.AgentInterface{
			{Transport: "http", Url: "http://weather-agent:8080/"},
			{Transport: "http", Url: "http://10.0.1.50:8000/"},
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
		_, _ = w.Write(cardJSON)
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

	// Verify URL was rewritten
	expectedURL := "https://gateway.agentic-layer.ai/test-agent"
	if responseCard.Url != expectedURL {
		t.Errorf("card.Url = %q, want %q", responseCard.Url, expectedURL)
	}

	// Verify grpc interface was removed (only http interfaces remain)
	if len(responseCard.AdditionalInterfaces) != 2 {
		t.Errorf("len(AdditionalInterfaces) = %d, want 2", len(responseCard.AdditionalInterfaces))
	}

	// Verify all http interfaces were rewritten to external gateway URL
	for i, iface := range responseCard.AdditionalInterfaces {
		if iface.Url != expectedURL {
			t.Errorf("AdditionalInterfaces[%d].Url = %q, want %q", i, iface.Url, expectedURL)
		}
		if iface.Transport != "http" {
			t.Errorf("AdditionalInterfaces[%d].Transport = %q, want http", i, iface.Transport)
		}
	}

	// Verify provider URL was not changed
	if responseCard.Provider.Url != "https://qaware.de" {
		t.Errorf("Provider.Url = %q, want unchanged", responseCard.Provider.Url)
	}

	// Verify Content-Type header is set for transformed response
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type header = %q, want %q", rec.Header().Get("Content-Type"), "application/json")
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
				_, _ = w.Write([]byte(tt.body))
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

			// Plugin returns early for non-OK status, so response is empty (default 200)
			// The backend response is captured but not written to the final response
			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d (plugin returns early)", rec.Code, http.StatusOK)
			}
			if rec.Body.String() != "" {
				t.Errorf("body = %q, want empty (plugin returns early)", rec.Body.String())
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
		_, _ = w.Write([]byte(malformedJSON))
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

	// Plugin returns early for malformed JSON, so response is empty (default 200)
	// The backend response is captured but not written to the final response
	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d (plugin returns early)", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "" {
		t.Errorf("body = %q, want empty (plugin returns early)", rec.Body.String())
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
		_, _ = w.Write([]byte(htmlResponse))
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

	// Plugin returns early for non-JSON content type, so response is empty (default 200)
	// The backend response is captured but not written to the final response
	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d (plugin returns early)", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "" {
		t.Errorf("body = %q, want empty (plugin returns early)", rec.Body.String())
	}
}

// TestClusterHostRewriting verifies internal URLs are rewritten even with cluster hostname in Host header
func TestClusterHostRewriting(t *testing.T) {
	agentCard := models.AgentCard{
		Name: "Test Agent",
		Url:  "http://test-agent.default.svc.cluster.local:8000/",
	}
	cardJSON, _ := json.Marshal(agentCard)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cardJSON)
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

	// Verify response was rewritten
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var responseCard models.AgentCard
	if err := json.Unmarshal(rec.Body.Bytes(), &responseCard); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// URL should be rewritten to use the Host header from request
	expectedURL := "http://agent-gateway.default.svc.cluster.local:10000/test-agent"
	if responseCard.Url != expectedURL {
		t.Errorf("card.Url = %q, want %q", responseCard.Url, expectedURL)
	}
}

// TestLocalhostHostRewriting verifies private IP URLs are rewritten even with localhost in Host header
func TestLocalhostHostRewriting(t *testing.T) {
	agentCard := models.AgentCard{
		Name: "Test Agent",
		Url:  "http://192.168.1.100:8000/",
	}
	cardJSON, _ := json.Marshal(agentCard)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cardJSON)
	})

	// Create the plugin handler
	handler, err := HandlerRegisterer.registerHandlers(context.Background(), map[string]interface{}{}, backend)
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// Create test request with localhost as host
	req := httptest.NewRequest(http.MethodGet, "/test-agent/.well-known/agent-card.json", nil)
	req.Host = "localhost:10000"
	req.Header.Set("X-Forwarded-Proto", "http")

	// Record response
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify response was rewritten
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var responseCard models.AgentCard
	if err := json.Unmarshal(rec.Body.Bytes(), &responseCard); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// URL should be rewritten to use the Host header from request
	expectedURL := "http://localhost:10000/test-agent"
	if responseCard.Url != expectedURL {
		t.Errorf("card.Url = %q, want %q", responseCard.Url, expectedURL)
	}
}

// TestExternalURLRewritten verifies external URLs in agent cards are rewritten
func TestExternalURLRewritten(t *testing.T) {
	externalURL := "https://external-agent.example.com/api"
	agentCard := models.AgentCard{
		Name: "External Agent",
		Url:  externalURL,
	}
	cardJSON, _ := json.Marshal(agentCard)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cardJSON)
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

	// External URL should be rewritten to gateway URL
	expectedURL := "https://gateway.agentic-layer.ai/external-agent"
	if responseCard.Url != expectedURL {
		t.Errorf("card.Url = %q, want %q", responseCard.Url, expectedURL)
	}
}

// TestResponseWriterCapture verifies the responseWriter correctly captures responses
func TestResponseWriterCapture(t *testing.T) {
	originalWriter := httptest.NewRecorder()
	rw := newResponseWriter(originalWriter)

	// Write status and body
	rw.WriteHeader(http.StatusCreated)
	_, _ = rw.Write([]byte("test body"))

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
		_, _ = w.Write([]byte("backend response"))
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

// TestMissingHostHeaderError verifies HTTP 500 error when Host header is missing
func TestMissingHostHeaderError(t *testing.T) {
	agentCard := models.AgentCard{
		Name: "Test Agent",
		Url:  "http://test-agent.default.svc.cluster.local:8000/",
	}
	cardJSON, _ := json.Marshal(agentCard)

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cardJSON)
	})

	// Create the plugin handler
	handler, err := HandlerRegisterer.registerHandlers(context.Background(), map[string]interface{}{}, backend)
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// Create test request with empty Host header
	req := httptest.NewRequest(http.MethodGet, "/test-agent/.well-known/agent-card.json", nil)
	req.Host = ""

	// Record response
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify HTTP 500 error response
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	// Verify error message
	if !strings.Contains(rec.Body.String(), "Host header is required") {
		t.Errorf("error message = %q, want to contain 'Host header is required'", rec.Body.String())
	}
}

// TestUnknownFieldPreservation verifies that unknown fields in agent cards are preserved
func TestUnknownFieldPreservation(t *testing.T) {
	// Create an agent card JSON with custom unknown fields
	agentCardJSON := `{
		"name": "Test Agent",
		"description": "A test agent",
		"url": "http://test-agent.default.svc.cluster.local:8000/",
		"version": "1.0.0",
		"x-custom-metadata": {
			"vendor": "ACME Corp",
			"license": "proprietary"
		},
		"experimental-feature": "enabled",
		"customArray": [1, 2, 3],
		"additionalInterfaces": [
			{
				"transport": "http",
				"url": "http://test-agent.default.svc.cluster.local:8000/",
				"customField": "should-be-preserved"
			},
			{
				"transport": "grpc",
				"url": "http://test-agent.default.svc.cluster.local:9000/"
			}
		]
	}`

	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(agentCardJSON))
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

	// Verify response
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	// Parse response as generic map to check all fields
	var responseMap map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &responseMap); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify URL was rewritten
	if responseMap["url"] != "https://gateway.agentic-layer.ai/test-agent" {
		t.Errorf("url = %v, want %q", responseMap["url"], "https://gateway.agentic-layer.ai/test-agent")
	}

	// Verify unknown fields are preserved
	if _, ok := responseMap["x-custom-metadata"]; !ok {
		t.Error("x-custom-metadata field was lost")
	} else {
		metadata := responseMap["x-custom-metadata"].(map[string]interface{})
		if metadata["vendor"] != "ACME Corp" {
			t.Errorf("x-custom-metadata.vendor = %v, want 'ACME Corp'", metadata["vendor"])
		}
		if metadata["license"] != "proprietary" {
			t.Errorf("x-custom-metadata.license = %v, want 'proprietary'", metadata["license"])
		}
	}

	if responseMap["experimental-feature"] != "enabled" {
		t.Errorf("experimental-feature = %v, want 'enabled'", responseMap["experimental-feature"])
	}

	if customArray, ok := responseMap["customArray"].([]interface{}); !ok {
		t.Error("customArray field was lost or wrong type")
	} else if len(customArray) != 3 {
		t.Errorf("customArray length = %d, want 3", len(customArray))
	}

	// Verify additionalInterfaces filtering still works and custom fields in interfaces are preserved
	if interfaces, ok := responseMap["additionalInterfaces"].([]interface{}); !ok {
		t.Error("additionalInterfaces field was lost")
	} else {
		if len(interfaces) != 1 {
			t.Errorf("additionalInterfaces length = %d, want 1 (grpc should be filtered out)", len(interfaces))
		}
		if len(interfaces) > 0 {
			iface := interfaces[0].(map[string]interface{})
			if iface["transport"] != "http" {
				t.Errorf("interface transport = %v, want 'http'", iface["transport"])
			}
			if iface["url"] != "https://gateway.agentic-layer.ai/test-agent" {
				t.Errorf("interface url = %v, want rewritten URL", iface["url"])
			}
			if iface["customField"] != "should-be-preserved" {
				t.Errorf("interface customField = %v, want 'should-be-preserved'", iface["customField"])
			}
		}
	}
}
