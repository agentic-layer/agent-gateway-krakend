package main

import (
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
