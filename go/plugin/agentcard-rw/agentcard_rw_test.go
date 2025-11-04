package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
	"github.com/stretchr/testify/assert"
)

// Test constants
const (
	testGatewayHost   = "gateway.agentic-layer.ai"
	testAgentCardPath = "/.well-known/agent-card.json"
	testHTTPSProtocol = "https"
	contentTypeJSON   = "application/json"
)

// testHelper provides utility methods for test setup
type testHelper struct {
	t *testing.T
}

func newTestHelper(t *testing.T) *testHelper {
	return &testHelper{t: t}
}

func (h *testHelper) createPluginHandler(backend http.HandlerFunc) http.Handler {
	handler, err := HandlerRegisterer.registerHandlers(context.Background(), map[string]interface{}{}, backend)
	if err != nil {
		h.t.Fatalf("failed to register handler: %v", err)
	}
	return handler
}

func (h *testHelper) makeRequest(handler http.Handler, method, path, host, protocol string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Host = host
	if protocol != "" {
		req.Header.Set("X-Forwarded-Proto", protocol)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func (h *testHelper) createAgentCardBackend(card models.AgentCard) http.HandlerFunc {
	cardJSON, _ := json.Marshal(card)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cardJSON)
	}
}

func (h *testHelper) createJSONBackend(jsonData string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(jsonData))
	}
}

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
			{Transport: "JSONRPC", Url: "http://weather-agent:8080/"},
			{Transport: "HTTP+JSON", Url: "http://10.0.1.50:8000/"},
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

	// Verify all valid transport interfaces are kept (JSONRPC, GRPC, HTTP+JSON)
	if len(responseCard.AdditionalInterfaces) != 3 {
		t.Errorf("len(AdditionalInterfaces) = %d, want 3", len(responseCard.AdditionalInterfaces))
	}

	// Verify all valid interfaces were rewritten to external gateway URL
	expectedTransports := []string{"JSONRPC", "HTTP+JSON", "grpc"}
	for i, iface := range responseCard.AdditionalInterfaces {
		if iface.Url != expectedURL {
			t.Errorf("AdditionalInterfaces[%d].Url = %q, want %q", i, iface.Url, expectedURL)
		}
		if iface.Transport != expectedTransports[i] {
			t.Errorf("AdditionalInterfaces[%d].Transport = %q, want %s", i, iface.Transport, expectedTransports[i])
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

// TestErrorConditionsPassThrough verifies various error conditions cause plugin to return early
func TestErrorConditionsPassThrough(t *testing.T) {
	tests := []struct {
		name               string
		statusCode         int
		contentType        string
		body               string
		description        string
		expectedStatusCode int
	}{
		{
			name:               "404 Not Found",
			statusCode:         http.StatusNotFound,
			contentType:        contentTypeJSON,
			body:               "not found",
			description:        "non-OK status codes",
			expectedStatusCode: http.StatusNotFound,
		},
		{
			name:               "500 Internal Server Error",
			statusCode:         http.StatusInternalServerError,
			contentType:        contentTypeJSON,
			body:               "internal error",
			description:        "non-OK status codes",
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			name:               "403 Forbidden",
			statusCode:         http.StatusForbidden,
			contentType:        contentTypeJSON,
			body:               "forbidden",
			description:        "non-OK status codes",
			expectedStatusCode: http.StatusForbidden,
		},
		{
			name:               "Malformed JSON",
			statusCode:         http.StatusOK,
			contentType:        contentTypeJSON,
			body:               `{"invalid": json}`,
			description:        "malformed JSON",
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			name:               "Non-JSON Content Type",
			statusCode:         http.StatusOK,
			contentType:        "text/html",
			body:               "<html><body>Not JSON</body></html>",
			description:        "non-JSON content types",
			expectedStatusCode: http.StatusUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			helper := newTestHelper(t)

			// Create backend that returns the error condition
			backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.Header().Set("X-Request-ID", "test-request-id")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			})

			handler := helper.createPluginHandler(backend)
			rec := helper.makeRequest(handler, http.MethodGet, "/test-agent"+testAgentCardPath, testGatewayHost, testHTTPSProtocol)

			assert.Equal(t, tt.expectedStatusCode, rec.Code)
			assert.NotEmpty(t, rec.Body)
		})
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
				"transport": "HTTP+JSON",
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
		if len(interfaces) != 2 {
			t.Errorf("additionalInterfaces length = %d, want 2 (both HTTP+JSON and grpc are valid)", len(interfaces))
		}
		if len(interfaces) > 0 {
			// First interface should be HTTP+JSON with custom field
			iface := interfaces[0].(map[string]interface{})
			if iface["transport"] != "HTTP+JSON" {
				t.Errorf("interface transport = %v, want 'HTTP+JSON'", iface["transport"])
			}
			if iface["url"] != "https://gateway.agentic-layer.ai/test-agent" {
				t.Errorf("interface url = %v, want rewritten URL", iface["url"])
			}
			if iface["customField"] != "should-be-preserved" {
				t.Errorf("interface customField = %v, want 'should-be-preserved'", iface["customField"])
			}
		}
		if len(interfaces) > 1 {
			// Second interface should be grpc
			iface := interfaces[1].(map[string]interface{})
			if iface["transport"] != "grpc" {
				t.Errorf("interface[1] transport = %v, want 'grpc'", iface["transport"])
			}
		}
	}
}

func TestGetGatewayURL(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		proto       string
		expected    string
		expectError bool
	}{
		{
			name:        "https with explicit proto header",
			host:        "gateway.agentic-layer.ai",
			proto:       "https",
			expected:    "https://gateway.agentic-layer.ai",
			expectError: false,
		},
		{
			name:        "default https without proto header",
			host:        "gateway.agentic-layer.ai",
			proto:       "",
			expected:    "http://gateway.agentic-layer.ai",
			expectError: false,
		},
		{
			name:        "http proto header with localhost",
			host:        "localhost:10000",
			proto:       "http",
			expected:    "http://localhost:10000",
			expectError: false,
		},
		{
			name:        "internal cluster host uses Host header",
			host:        "agent-gateway.default.svc.cluster.local",
			proto:       "https",
			expected:    "https://agent-gateway.default.svc.cluster.local",
			expectError: false,
		},
		{
			name:        "host with port",
			host:        "gateway.agentic-layer.ai:443",
			proto:       "https",
			expected:    "https://gateway.agentic-layer.ai:443",
			expectError: false,
		},
		{
			name:        "internal cluster variant with port",
			host:        "service.namespace.svc.cluster.local:8080",
			proto:       "http",
			expected:    "http://service.namespace.svc.cluster.local:8080",
			expectError: false,
		},
		{
			name:        "external gateway host",
			host:        "gateway-from-header.example.com",
			proto:       "https",
			expected:    "https://gateway-from-header.example.com",
			expectError: false,
		},
		{
			name:        "docker internal hostname",
			host:        "host.docker.internal",
			proto:       "http",
			expected:    "http://host.docker.internal",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Host:   tt.host,
				Header: http.Header{},
			}
			if tt.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.proto)
			}

			result, err := getGatewayURL(req)

			if tt.expectError {
				if err == nil {
					t.Errorf("getGatewayURL() expected error but got none, result = %q", result)
				}
			} else {
				if err != nil {
					t.Errorf("getGatewayURL() unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("getGatewayURL() = %q, want %q", result, tt.expected)
				}
			}
		})
	}
}
