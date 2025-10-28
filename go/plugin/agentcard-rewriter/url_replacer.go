package main

import (
	"strings"

	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
)

// constructExternalURL builds the external gateway URL from gateway URL and agent path
func constructExternalURL(gatewayURL string, agentPath string) string {
	// Remove trailing slashes
	gatewayURL = strings.TrimSuffix(gatewayURL, "/")
	agentPath = strings.TrimSuffix(agentPath, "/")

	return gatewayURL + agentPath
}

// checkProviderURL checks if the provider URL is suspiciously internal
// Returns (shouldWarn, reason)
func checkProviderURL(providerURL string) (bool, string) {
	if providerURL == "" {
		return false, ""
	}

	if isInternalURL(providerURL) {
		return true, "contains .svc.cluster.local"
	}

	return false, ""
}

// rewriteAdditionalInterfaces filters and rewrites additional interfaces
// - Keeps only HTTP/HTTPS transports
// - Rewrites internal URLs to external gateway URLs
// - Removes unsupported transports (gRPC, WebSocket, SSE, etc.)
func rewriteAdditionalInterfaces(interfaces []models.AgentInterface, gatewayURL string, agentPath string) []models.AgentInterface {
	var result []models.AgentInterface
	externalURL := constructExternalURL(gatewayURL, agentPath)

	for _, iface := range interfaces {
		// Only keep http and https transports
		if iface.Transport == "http" || iface.Transport == "https" {
			// Rewrite internal URLs
			if isInternalURL(iface.Url) {
				iface.Url = externalURL
			}
			result = append(result, iface)
		}
		// All other transports are implicitly removed
	}

	return result
}

// rewriteAgentCard transforms internal URLs to external gateway URLs in an agent card
func rewriteAgentCard(card models.AgentCard, gatewayURL string, agentPath string) models.AgentCard {
	externalURL := constructExternalURL(gatewayURL, agentPath)

	// Rewrite main URL if it's internal
	if isInternalURL(card.Url) {
		card.Url = externalURL
	}

	// Rewrite and filter additional interfaces
	card.AdditionalInterfaces = rewriteAdditionalInterfaces(card.AdditionalInterfaces, gatewayURL, agentPath)

	// Provider URL is never rewritten (it's organizational metadata, not an agent endpoint)

	return card
}
