package main

import (
	"fmt"
	"net/http"
	"strings"
)

// agentCardSuffix is the standard path suffix for agent card endpoints
const agentCardSuffix = "/.well-known/agent-card.json"

// isAgentCardEndpoint checks if the path matches the agent card endpoint pattern
func isAgentCardEndpoint(path string) bool {
	return strings.HasSuffix(path, agentCardSuffix)
}

// extractAgentPath extracts the full agent path from the request path (everything before the agent card suffix)
// Examples:
//
//	"/weather-agent/.well-known/agent-card.json" -> "/weather-agent"
//	"/agents/weather-agent/.well-known/agent-card.json" -> "/agents/weather-agent"
//	"/api/v1/agents/weather-agent/.well-known/agent-card.json" -> "/api/v1/agents/weather-agent"
//	"/.well-known/weather-agent/.well-known/agent-card.json" -> "/.well-known/weather-agent"
func extractAgentPath(path string) string {
	// Find the position of the agent card suffix
	idx := strings.Index(path, agentCardSuffix)
	if idx > 0 {
		return path[:idx]
	}

	// If suffix is at the start or not found, return empty
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
