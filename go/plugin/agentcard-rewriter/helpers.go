package main

import (
	"fmt"
	"net/http"
	"strings"
)

// isAgentCardEndpoint checks if the path matches the agent card endpoint pattern
func isAgentCardEndpoint(path string) bool {
	return strings.HasSuffix(path, "/.well-known/agent-card.json")
}

// extractAgentPath extracts the full agent path from the request path (everything before /.well-known)
// Examples:
//
//	"/weather-agent/.well-known/agent-card.json" -> "/weather-agent"
//	"/agents/weather-agent/.well-known/agent-card.json" -> "/agents/weather-agent"
//	"/api/v1/agents/weather-agent/.well-known/agent-card.json" -> "/api/v1/agents/weather-agent"
func extractAgentPath(path string) string {
	// Find the position of /.well-known
	idx := strings.Index(path, "/.well-known")
	if idx > 0 {
		return path[:idx]
	}

	// If /.well-known is at the start or not found, return empty
	return ""
}

// getGatewayURL extracts the gateway URL from request headers
// Returns the full URL scheme + host, or an error if Host header is missing
func getGatewayURL(req *http.Request) (string, error) {
	host := req.Host

	if host == "" {
		return "", fmt.Errorf("Host header is required for agent card URL rewriting")
	}

	// Default to https, but check X-Forwarded-Proto header
	scheme := "https"
	if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	return fmt.Sprintf("%s://%s", scheme, host), nil
}

// copyHeaders copies all headers from source to destination
func copyHeaders(dst, src http.Header) {
	for k, v := range src {
		dst[k] = v
	}
}
