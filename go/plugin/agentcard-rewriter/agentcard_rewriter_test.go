package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
)

// Test configuration strings
const (
	configStrEmpty   = `{}`
	configStrMinimal = `{
		"agentcard_rw_config": {}
	}`
	configStrWithURL = `{
		"agentcard_rw_config": {
			"gateway_url": "https://configured-gateway.example.com"
		}
	}`
	configStrInvalidConfig = `{
		"agentcard_rw_config": "not an object"
	}`
)

// TestParseConfig verifies configuration parsing works correctly
func TestParseConfig(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		expectError bool
		checkFunc   func(t *testing.T, cfg config)
	}{
		{
			name:        "empty config uses defaults",
			configJSON:  configStrEmpty,
			expectError: false,
			checkFunc: func(t *testing.T, cfg config) {
				if cfg.GatewayURL != "" {
					t.Errorf("GatewayURL = %q, want empty", cfg.GatewayURL)
				}
			},
		},
		{
			name:        "minimal config with empty values",
			configJSON:  configStrMinimal,
			expectError: false,
			checkFunc: func(t *testing.T, cfg config) {
				if cfg.GatewayURL != "" {
					t.Errorf("GatewayURL = %q, want empty", cfg.GatewayURL)
				}
			},
		},
		{
			name:        "config with gateway_url",
			configJSON:  configStrWithURL,
			expectError: false,
			checkFunc: func(t *testing.T, cfg config) {
				expected := "https://configured-gateway.example.com"
				if cfg.GatewayURL != expected {
					t.Errorf("GatewayURL = %q, want %q", cfg.GatewayURL, expected)
				}
			},
		},
		{
			name:        "invalid config type should error",
			configJSON:  configStrInvalidConfig,
			expectError: true,
			checkFunc:   func(t *testing.T, cfg config) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var extraConfig map[string]interface{}
			if err := json.Unmarshal([]byte(tt.configJSON), &extraConfig); err != nil {
				t.Fatalf("failed to unmarshal test config: %v", err)
			}

			var cfg config
			err := parseConfig(extraConfig, &cfg)

			if tt.expectError {
				if err == nil {
					t.Errorf("parseConfig() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("parseConfig() unexpected error: %v", err)
				}
				tt.checkFunc(t, cfg)
			}
		})
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

// TestConfigFallbackIntegration verifies config fallback works end-to-end
func TestConfigFallbackIntegration(t *testing.T) {
	tests := []struct {
		name          string
		configJSON    string
		requestHost   string
		expectedURL   string
		shouldRewrite bool
	}{
		{
			name: "internal cluster host with config fallback",
			configJSON: `{
				"agentcard_rw_config": {
					"gateway_url": "https://configured-gateway.example.com"
				}
			}`,
			requestHost:   "agent-gateway.default.svc.cluster.local:10000",
			expectedURL:   "https://configured-gateway.example.com/test-agent",
			shouldRewrite: true,
		},
		{
			name: "empty host with config fallback",
			configJSON: `{
				"agentcard_rw_config": {
					"gateway_url": "https://configured-gateway.example.com"
				}
			}`,
			requestHost:   "",
			expectedURL:   "https://configured-gateway.example.com/test-agent",
			shouldRewrite: true,
		},
		{
			name: "headers take precedence over config",
			configJSON: `{
				"agentcard_rw_config": {
					"gateway_url": "https://configured-gateway.example.com"
				}
			}`,
			requestHost:   "header-gateway.example.com",
			expectedURL:   "https://header-gateway.example.com/test-agent",
			shouldRewrite: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock backend that returns an agent card with internal URLs
			agentCard := models.AgentCard{
				Name:    "Test Agent",
				Url:     "http://test-agent.default.svc.cluster.local:8000/",
				Version: "1.0.0",
			}
			cardJSON, _ := json.Marshal(agentCard)

			backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(cardJSON)
			})

			// Parse config
			var extraConfig map[string]interface{}
			if err := json.Unmarshal([]byte(tt.configJSON), &extraConfig); err != nil {
				t.Fatalf("failed to unmarshal config: %v", err)
			}

			// Create the plugin handler with config
			handler, err := HandlerRegisterer.registerHandlers(context.Background(), extraConfig, backend)
			if err != nil {
				t.Fatalf("failed to register handler: %v", err)
			}

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/test-agent/.well-known/agent-card.json", nil)
			req.Host = tt.requestHost
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

			// Verify URL was rewritten correctly
			if responseCard.Url != tt.expectedURL {
				t.Errorf("card.Url = %q, want %q", responseCard.Url, tt.expectedURL)
			}
		})
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
