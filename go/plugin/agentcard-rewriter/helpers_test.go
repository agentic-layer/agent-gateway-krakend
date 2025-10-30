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

func TestExtractAgentPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "weather agent",
			path:     "/weather-agent/.well-known/agent-card.json",
			expected: "/weather-agent",
		},
		{
			name:     "cross-selling agent",
			path:     "/cross-selling-agent/.well-known/agent-card.json",
			expected: "/cross-selling-agent",
		},
		{
			name:     "agent with multiple hyphens",
			path:     "/agent-name-with-hyphens/.well-known/agent-card.json",
			expected: "/agent-name-with-hyphens",
		},
		{
			name:     "insurance host agent",
			path:     "/insurance-host-agent/.well-known/agent-card.json",
			expected: "/insurance-host-agent",
		},
		{
			name:     "nested path with multiple segments",
			path:     "/agents/weather-agent/.well-known/agent-card.json",
			expected: "/agents/weather-agent",
		},
		{
			name:     "deeply nested path",
			path:     "/api/v1/agents/weather-agent/.well-known/agent-card.json",
			expected: "/api/v1/agents/weather-agent",
		},
		{
			name:     "root well-known",
			path:     "/.well-known/agent-card.json",
			expected: "",
		},
		{
			name:     "path without well-known",
			path:     "/weather-agent/some/other/path",
			expected: "",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAgentPath(tt.path)
			if result != tt.expected {
				t.Errorf("extractAgentPath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetGatewayURL(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		proto       string
		cfg         config
		expected    string
		expectError bool
	}{
		{
			name:        "https with explicit proto header",
			host:        "gateway.agentic-layer.ai",
			proto:       "https",
			cfg:         config{},
			expected:    "https://gateway.agentic-layer.ai",
			expectError: false,
		},
		{
			name:        "default https without proto header",
			host:        "gateway.agentic-layer.ai",
			proto:       "",
			cfg:         config{},
			expected:    "https://gateway.agentic-layer.ai",
			expectError: false,
		},
		{
			name:        "http proto header with localhost requires config",
			host:        "localhost:10000",
			proto:       "http",
			cfg:         config{},
			expected:    "",
			expectError: true, // localhost is now detected as internal
		},
		{
			name:        "internal cluster host with no config fallback should error",
			host:        "agent-gateway.default.svc.cluster.local",
			proto:       "https",
			cfg:         config{},
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty host with no config fallback should error",
			host:        "",
			proto:       "https",
			cfg:         config{},
			expected:    "",
			expectError: true,
		},
		{
			name:        "host with port",
			host:        "gateway.agentic-layer.ai:443",
			proto:       "https",
			cfg:         config{},
			expected:    "https://gateway.agentic-layer.ai:443",
			expectError: false,
		},
		{
			name:        "another internal cluster variant with no config",
			host:        "service.namespace.svc.cluster.local:8080",
			proto:       "http",
			cfg:         config{},
			expected:    "",
			expectError: true,
		},
		{
			name:  "internal cluster host with config fallback",
			host:  "agent-gateway.default.svc.cluster.local",
			proto: "https",
			cfg: config{
				GatewayURL: "https://gateway.agentic-layer.ai",
			},
			expected:    "https://gateway.agentic-layer.ai",
			expectError: false,
		},
		{
			name:  "empty host with config fallback",
			host:  "",
			proto: "https",
			cfg: config{
				GatewayURL: "https://configured-gateway.example.com",
			},
			expected:    "https://configured-gateway.example.com",
			expectError: false,
		},
		{
			name:  "headers take precedence over config",
			host:  "gateway-from-header.example.com",
			proto: "https",
			cfg: config{
				GatewayURL: "https://gateway-from-config.example.com",
			},
			expected:    "https://gateway-from-header.example.com",
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

			result, err := getGatewayURL(req, tt.cfg)

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

func TestIsInternalURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		// Kubernetes cluster URLs (existing tests)
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
		// Kubernetes short-form service names (NEW)
		{
			name:     "k8s short-form service name (same namespace)",
			url:      "http://weather-agent:8000",
			expected: true,
		},
		{
			name:     "k8s short-form service name (cross-namespace)",
			url:      "http://weather-agent.default:8000",
			expected: true,
		},
		{
			name:     "k8s service with .svc only",
			url:      "http://agent-runtime.svc:8080",
			expected: true,
		},
		{
			name:     "k8s service with namespace and .svc",
			url:      "http://agent-runtime.agentic-platform.svc",
			expected: true,
		},
		// Localhost and loopback (NEW - updated expectations)
		{
			name:     "localhost URL",
			url:      "http://localhost:8000",
			expected: true, // Changed from false
		},
		{
			name:     "localhost https",
			url:      "https://localhost:3000/api",
			expected: true,
		},
		{
			name:     "IPv4 loopback 127.0.0.1",
			url:      "http://127.0.0.1:8080",
			expected: true,
		},
		{
			name:     "IPv4 loopback 127.x.x.x range",
			url:      "http://127.1.2.3:8080",
			expected: true,
		},
		{
			name:     "IPv6 loopback",
			url:      "http://[::1]:8080",
			expected: true,
		},
		// Private IPv4 ranges (NEW - updated expectations)
		{
			name:     "private IP 192.168.x.x",
			url:      "http://192.168.1.1:8080",
			expected: true, // Changed from false
		},
		{
			name:     "private IP 10.x.x.x (typical K8s ClusterIP)",
			url:      "http://10.96.10.45:8000",
			expected: true,
		},
		{
			name:     "private IP 10.x.x.x without port",
			url:      "http://10.0.0.1",
			expected: true,
		},
		{
			name:     "private IP 172.16-31.x.x",
			url:      "http://172.17.0.2:8080",
			expected: true,
		},
		{
			name:     "private IP 172.20.x.x (mid-range)",
			url:      "http://172.20.5.10:3000",
			expected: true,
		},
		// Link-local addresses (NEW)
		{
			name:     "link-local IPv4 169.254.x.x",
			url:      "http://169.254.1.1:8080",
			expected: true,
		},
		{
			name:     "link-local IPv6",
			url:      "http://[fe80::1]:8000",
			expected: true,
		},
		// Docker internal hostnames (NEW)
		{
			name:     "Docker host.docker.internal",
			url:      "http://host.docker.internal:8000",
			expected: true,
		},
		{
			name:     "Docker gateway.docker.internal",
			url:      "http://gateway.docker.internal:3000",
			expected: true,
		},
		// Special-use domains (NEW)
		{
			name:     "mDNS .local domain",
			url:      "http://myservice.local:8000",
			expected: true,
		},
		{
			name:     ".internal domain",
			url:      "http://agent.internal:3000",
			expected: true,
		},
		{
			name:     ".localhost domain",
			url:      "http://test.localhost:8080",
			expected: true,
		},
		// IPv6 ULA (NEW)
		{
			name:     "IPv6 ULA fc00::/7",
			url:      "http://[fd12:3456:789a:1::1]:8000",
			expected: true,
		},
		{
			name:     "IPv6 ULA fcXX range",
			url:      "http://[fc00::1]:8000",
			expected: true,
		},
		// External URLs (should remain false)
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
			name:     "external API with subdomain",
			url:      "https://api.example.com/v1/agents",
			expected: false,
		},
		{
			name:     "public IP address",
			url:      "http://8.8.8.8:80",
			expected: false,
		},
		{
			name:     "public IPv6 address",
			url:      "http://[2001:4860:4860::8888]:80",
			expected: false,
		},
		// Edge cases
		{
			name:     "empty URL",
			url:      "",
			expected: false,
		},
		{
			name:     "partial cluster domain (missing .svc)",
			url:      "http://my-service.cluster.local:8000",
			expected: false,
		},
		{
			name:     "svc but not cluster local (public domain)",
			url:      "http://my-service.svc.example.com",
			expected: false,
		},
		{
			name:     "malformed URL",
			url:      "not-a-valid-url",
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
