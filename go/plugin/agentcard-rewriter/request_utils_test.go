package main

import (
	"net/http"
	"testing"
)

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
