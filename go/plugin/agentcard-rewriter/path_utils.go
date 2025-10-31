package main

import (
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
