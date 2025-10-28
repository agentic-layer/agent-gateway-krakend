package main

import (
	"net/http"
	"testing"
)

func TestIsAgentCardEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "root well-known agent card",
			path:     "/.well-known/agent-card.json",
			expected: true,
		},
		{
			name:     "agent with well-known agent card",
			path:     "/weather-agent/.well-known/agent-card.json",
			expected: true,
		},
		{
			name:     "cross-selling agent card",
			path:     "/cross-selling-agent/.well-known/agent-card.json",
			expected: true,
		},
		{
			name:     "agent with hyphens",
			path:     "/agent-name-with-hyphens/.well-known/agent-card.json",
			expected: true,
		},
		{
			name:     "not well-known endpoint",
			path:     "/agent-card.json",
			expected: false,
		},
		{
			name:     "api endpoint",
			path:     "/api/agents",
			expected: false,
		},
		{
			name:     "chat completions endpoint",
			path:     "/weather-agent/chat/completions",
			expected: false,
		},
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
		{
			name:     "root path",
			path:     "/",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAgentCardEndpoint(tt.path)
			if result != tt.expected {
				t.Errorf("isAgentCardEndpoint(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestExtractAgentName(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "weather agent",
			path:     "/weather-agent/.well-known/agent-card.json",
			expected: "weather-agent",
		},
		{
			name:     "cross-selling agent",
			path:     "/cross-selling-agent/.well-known/agent-card.json",
			expected: "cross-selling-agent",
		},
		{
			name:     "agent with multiple hyphens",
			path:     "/agent-name-with-hyphens/.well-known/agent-card.json",
			expected: "agent-name-with-hyphens",
		},
		{
			name:     "insurance host agent",
			path:     "/insurance-host-agent/.well-known/agent-card.json",
			expected: "insurance-host-agent",
		},
		{
			name:     "root well-known",
			path:     "/.well-known/agent-card.json",
			expected: "",
		},
		{
			name:     "single path component",
			path:     "/single",
			expected: "single",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "just slash",
			path:     "/",
			expected: "",
		},
		{
			name:     "agent without well-known",
			path:     "/weather-agent/some/other/path",
			expected: "weather-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAgentName(tt.path)
			if result != tt.expected {
				t.Errorf("extractAgentName(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetGatewayDomain(t *testing.T) {
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
			expected:    "https://gateway.agentic-layer.ai",
			expectError: false,
		},
		{
			name:        "http proto header",
			host:        "localhost:10000",
			proto:       "http",
			expected:    "http://localhost:10000",
			expectError: false,
		},
		{
			name:        "internal cluster host should error",
			host:        "agent-gateway.default.svc.cluster.local",
			proto:       "https",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty host should error",
			host:        "",
			proto:       "https",
			expected:    "",
			expectError: true,
		},
		{
			name:        "host with port",
			host:        "gateway.agentic-layer.ai:443",
			proto:       "https",
			expected:    "https://gateway.agentic-layer.ai:443",
			expectError: false,
		},
		{
			name:        "another internal cluster variant",
			host:        "service.namespace.svc.cluster.local:8080",
			proto:       "http",
			expected:    "",
			expectError: true,
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

			result, err := getGatewayDomain(req)

			if tt.expectError {
				if err == nil {
					t.Errorf("getGatewayDomain() expected error but got none, result = %q", result)
				}
			} else {
				if err != nil {
					t.Errorf("getGatewayDomain() unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("getGatewayDomain() = %q, want %q", result, tt.expected)
				}
			}
		})
	}
}

func TestIsInternalURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "internal cluster URL with namespace",
			url:      "http://agent.default.svc.cluster.local:8000/",
			expected: true,
		},
		{
			name:     "internal cluster URL without port",
			url:      "http://agent.svc.cluster.local/",
			expected: true,
		},
		{
			name:     "internal cluster URL https",
			url:      "https://service.namespace.svc.cluster.local:443/path",
			expected: true,
		},
		{
			name:     "external gateway URL",
			url:      "https://gateway.agentic-layer.ai/weather-agent",
			expected: false,
		},
		{
			name:     "external company URL",
			url:      "https://qaware.de",
			expected: false,
		},
		{
			name:     "localhost URL",
			url:      "http://localhost:8000",
			expected: false,
		},
		{
			name:     "IP address URL",
			url:      "http://192.168.1.1:8080",
			expected: false,
		},
		{
			name:     "empty URL",
			url:      "",
			expected: false,
		},
		{
			name:     "partial cluster domain",
			url:      "http://my-service.cluster.local:8000",
			expected: false,
		},
		{
			name:     "svc but not cluster local",
			url:      "http://my-service.svc.example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInternalURL(tt.url)
			if result != tt.expected {
				t.Errorf("isInternalURL(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestCopyHeaders(t *testing.T) {
	tests := []struct {
		name        string
		srcHeaders  map[string][]string
		dstHeaders  map[string][]string
		expectedDst map[string][]string
	}{
		{
			name: "copy single header",
			srcHeaders: map[string][]string{
				"X-Request-ID": {"test-123"},
			},
			dstHeaders: map[string][]string{},
			expectedDst: map[string][]string{
				"X-Request-ID": {"test-123"},
			},
		},
		{
			name: "copy multiple headers",
			srcHeaders: map[string][]string{
				"X-Request-ID":  {"test-123"},
				"Cache-Control": {"max-age=3600"},
				"Content-Type":  {"application/json"},
			},
			dstHeaders: map[string][]string{},
			expectedDst: map[string][]string{
				"X-Request-ID":  {"test-123"},
				"Cache-Control": {"max-age=3600"},
				"Content-Type":  {"application/json"},
			},
		},
		{
			name: "override existing headers",
			srcHeaders: map[string][]string{
				"Content-Type": {"application/json"},
				"X-Custom":     {"new-value"},
			},
			dstHeaders: map[string][]string{
				"Content-Type": {"text/html"},
				"X-Existing":   {"existing-value"},
			},
			expectedDst: map[string][]string{
				"Content-Type": {"application/json"},
				"X-Custom":     {"new-value"},
				"X-Existing":   {"existing-value"},
			},
		},
		{
			name: "copy multi-value headers",
			srcHeaders: map[string][]string{
				"Set-Cookie": {"session=abc123", "user=john"},
				"X-Custom":   {"value1", "value2"},
			},
			dstHeaders: map[string][]string{},
			expectedDst: map[string][]string{
				"Set-Cookie": {"session=abc123", "user=john"},
				"X-Custom":   {"value1", "value2"},
			},
		},
		{
			name:       "copy from empty source",
			srcHeaders: map[string][]string{},
			dstHeaders: map[string][]string{
				"X-Existing": {"existing-value"},
			},
			expectedDst: map[string][]string{
				"X-Existing": {"existing-value"},
			},
		},
		{
			name: "copy to empty destination",
			srcHeaders: map[string][]string{
				"X-Source": {"source-value"},
			},
			dstHeaders: map[string][]string{},
			expectedDst: map[string][]string{
				"X-Source": {"source-value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := http.Header(tt.srcHeaders)
			dst := http.Header(tt.dstHeaders)

			copyHeaders(dst, src)

			// Verify all expected headers are present
			for key, expectedValues := range tt.expectedDst {
				actualValues := dst[key]
				if len(actualValues) != len(expectedValues) {
					t.Errorf("header %q has %d values, want %d", key, len(actualValues), len(expectedValues))
					continue
				}
				for i, expectedValue := range expectedValues {
					if actualValues[i] != expectedValue {
						t.Errorf("header %q[%d] = %q, want %q", key, i, actualValues[i], expectedValue)
					}
				}
			}

			// Verify no unexpected headers are present
			if len(dst) != len(tt.expectedDst) {
				t.Errorf("destination has %d headers, want %d", len(dst), len(tt.expectedDst))
			}
		})
	}
}
