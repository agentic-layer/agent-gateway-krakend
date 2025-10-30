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
		return true, "contains internal/local URL"
	}

	return false, ""
}

// safeGetString safely extracts a string value from a map
func safeGetString(m map[string]interface{}, key string) (string, bool) {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}

// safeGetArray safely extracts an array value from a map
func safeGetArray(m map[string]interface{}, key string) ([]interface{}, bool) {
	if val, ok := m[key]; ok {
		if arr, ok := val.([]interface{}); ok {
			return arr, true
		}
	}
	return nil, false
}

// safeGetMap safely extracts a map value from a map
func safeGetMap(m map[string]interface{}, key string) (map[string]interface{}, bool) {
	if val, ok := m[key]; ok {
		if subMap, ok := val.(map[string]interface{}); ok {
			return subMap, true
		}
	}
	return nil, false
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
				// TODO: Technical decision needed: Should we normalize transport field to match
				// external URL scheme (https), or deduplicate interfaces when they all point to
				// the same URL? Currently all internal URLs are rewritten to the same external
				// gateway URL regardless of original transport (http/https/other), which may be
				// misleading as transport field doesn't match the URL scheme.
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

// rewriteAdditionalInterfacesMap filters and rewrites additional interfaces using map representation
// - Keeps only HTTP/HTTPS transports
// - Rewrites internal URLs to external gateway URLs
// - Removes unsupported transports (gRPC, WebSocket, SSE, etc.)
// - Preserves all other fields in the interface objects
func rewriteAdditionalInterfacesMap(interfaces []interface{}, gatewayURL string, agentPath string) []interface{} {
	var result []interface{}
	externalURL := constructExternalURL(gatewayURL, agentPath)

	for _, iface := range interfaces {
		// Try to cast to map
		ifaceMap, ok := iface.(map[string]interface{})
		if !ok {
			continue // Skip non-map entries
		}

		// Get transport type
		transport, ok := safeGetString(ifaceMap, "transport")
		if !ok {
			continue // Skip entries without transport
		}

		// Only keep http and https transports
		if transport == "http" || transport == "https" {
			// Get URL
			if url, ok := safeGetString(ifaceMap, "url"); ok {
				// Rewrite internal URLs
				if isInternalURL(url) {
					// TODO: Technical decision needed: Should we normalize transport field to match
					// external URL scheme (https), or deduplicate interfaces when they all point to
					// the same URL? Currently all internal URLs are rewritten to the same external
					// gateway URL regardless of original transport (http/https/other), which may be
					// misleading as transport field doesn't match the URL scheme.
					ifaceMap["url"] = externalURL
				}
			}
			result = append(result, ifaceMap)
		}
		// All other transports are implicitly removed
	}

	return result
}

// rewriteAgentCardMap transforms internal URLs to external gateway URLs in an agent card map
// This function preserves all unknown fields in the agent card
func rewriteAgentCardMap(cardMap map[string]interface{}, gatewayURL string, agentPath string) map[string]interface{} {
	externalURL := constructExternalURL(gatewayURL, agentPath)

	// Rewrite main URL if it's internal
	if url, ok := safeGetString(cardMap, "url"); ok {
		if isInternalURL(url) {
			cardMap["url"] = externalURL
		}
	}

	// Rewrite and filter additional interfaces
	if interfaces, ok := safeGetArray(cardMap, "additionalInterfaces"); ok {
		cardMap["additionalInterfaces"] = rewriteAdditionalInterfacesMap(interfaces, gatewayURL, agentPath)
	}

	// Provider URL is never rewritten (it's organizational metadata, not an agent endpoint)

	return cardMap
}
