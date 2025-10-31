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
		{
			name:     "agent path starting with well-known",
			path:     "/.well-known/weather-agent/.well-known/agent-card.json",
			expected: true,
		},
		{
			name:     "multiple well-known segments in path",
			path:     "/api/.well-known/agents/weather/.well-known/agent-card.json",
			expected: true,
		},
		{
			name:     "agent name contains well-known substring",
			path:     "/my-.well-known-agent/.well-known/agent-card.json",
			expected: true,
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
		{
			name:     "agent path starting with well-known",
			path:     "/.well-known/weather-agent/.well-known/agent-card.json",
			expected: "/.well-known/weather-agent",
		},
		{
			name:     "multiple well-known segments in path",
			path:     "/api/.well-known/agents/weather/.well-known/agent-card.json",
			expected: "/api/.well-known/agents/weather",
		},
		{
			name:     "agent name contains well-known substring",
			path:     "/my-.well-known-agent/.well-known/agent-card.json",
			expected: "/my-.well-known-agent",
		},
		{
			name:     "well-known as agent name",
			path:     "/.well-known/.well-known/agent-card.json",
			expected: "/.well-known",
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
		{ // todo does this example make sense?
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
