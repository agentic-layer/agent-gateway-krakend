package main

import (
	"strings"

	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
)

// constructExternalURL builds the external gateway URL from domain, agent name, and optional path prefix
func constructExternalURL(gatewayDomain string, agentName string, pathPrefix string) string {
	// Remove trailing slashes
	gatewayDomain = strings.TrimSuffix(gatewayDomain, "/")
	pathPrefix = strings.TrimSuffix(pathPrefix, "/")

	// Build URL with optional path prefix
	if pathPrefix != "" {
		return gatewayDomain + pathPrefix + "/" + agentName
	}
	return gatewayDomain + "/" + agentName
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
func rewriteAdditionalInterfaces(interfaces []models.AgentInterface, gatewayDomain string, agentName string, pathPrefix string) []models.AgentInterface {
	var result []models.AgentInterface
	externalURL := constructExternalURL(gatewayDomain, agentName, pathPrefix)

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
func rewriteAgentCard(card models.AgentCard, gatewayDomain string, agentName string, pathPrefix string) models.AgentCard {
	externalURL := constructExternalURL(gatewayDomain, agentName, pathPrefix)

	// Rewrite main URL if it's internal
	if isInternalURL(card.Url) {
		card.Url = externalURL
	}

	// Rewrite and filter additional interfaces
	card.AdditionalInterfaces = rewriteAdditionalInterfaces(card.AdditionalInterfaces, gatewayDomain, agentName, pathPrefix)

	// Provider URL is never rewritten (it's organizational metadata, not an agent endpoint)

	return card
}
