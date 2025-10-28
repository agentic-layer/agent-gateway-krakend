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

// extractAgentName extracts the agent name from the request path
// Example: "/weather-agent/.well-known/agent-card.json" -> "weather-agent"
func extractAgentName(path string) string {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Empty path after trimming
	if path == "" {
		return ""
	}

	// Split by slash and get first component
	parts := strings.Split(path, "/")
	if len(parts) > 0 && parts[0] != "" {
		// If the first part is .well-known, there's no agent name
		if parts[0] == ".well-known" {
			return ""
		}
		return parts[0]
	}

	return ""
}

// getGatewayDomain extracts the gateway domain from request headers with config fallback
// Returns the full URL scheme + host, or an error if it cannot be determined
func getGatewayDomain(req *http.Request, cfg config) (string, error) {
	host := req.Host

	// Try header detection first
	if host != "" && !strings.Contains(host, ".svc.cluster.local") {
		// Default to https, but check X-Forwarded-Proto header
		scheme := "https"
		if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		}
		return fmt.Sprintf("%s://%s", scheme, host), nil
	}

	// Fallback to configured gateway domain
	if cfg.GatewayDomain != "" {
		return cfg.GatewayDomain, nil
	}

	// Both methods failed
	if host == "" {
		return "", fmt.Errorf("Host header not present and no gateway_domain configured")
	}
	return "", fmt.Errorf("internal cluster request and no gateway_domain configured")
}

// isInternalURL checks if a URL is an internal Kubernetes cluster URL
func isInternalURL(url string) bool {
	return strings.Contains(url, ".svc.cluster.local")
}

// copyHeaders copies all headers from source to destination
func copyHeaders(dst, src http.Header) {
	for k, v := range src {
		dst[k] = v
	}
}
