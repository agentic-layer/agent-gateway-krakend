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
// - Rewrites all URLs to external gateway URLs
// - Removes unsupported transports (gRPC, WebSocket, SSE, etc.)
func rewriteAdditionalInterfaces(interfaces []models.AgentInterface, gatewayURL string, agentPath string) []models.AgentInterface {
	var result []models.AgentInterface
	externalURL := constructExternalURL(gatewayURL, agentPath)

	for _, iface := range interfaces {
		// Only keep http and https transports
		if iface.Transport == "http" || iface.Transport == "https" {
			iface.Url = externalURL
			result = append(result, iface)
		}
		// All other transports are implicitly removed
	}

	return result
}

// rewriteAgentCard transforms all URLs to external gateway URLs in an agent card
func rewriteAgentCard(card models.AgentCard, gatewayURL string, agentPath string) models.AgentCard {
	externalURL := constructExternalURL(gatewayURL, agentPath)

	// Rewrite main URL
	card.Url = externalURL

	// Rewrite and filter additional interfaces
	card.AdditionalInterfaces = rewriteAdditionalInterfaces(card.AdditionalInterfaces, gatewayURL, agentPath)

	// Provider URL is never rewritten (it's organizational metadata, not an agent endpoint)

	return card
}

// rewriteAdditionalInterfacesMap filters and rewrites additional interfaces using map representation
// - Keeps only HTTP/HTTPS transports
// - Rewrites URLs to external gateway URLs
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
			// Rewrite URLs to gateway URL
			if _, ok := safeGetString(ifaceMap, "url"); ok {
				ifaceMap["url"] = externalURL
			}
			result = append(result, ifaceMap)
		}
		// All other transports are implicitly removed
	}

	return result
}

// rewriteAgentCardMap transforms URLs to external gateway URLs in an agent card map
// This function preserves all unknown fields in the agent card
func rewriteAgentCardMap(cardMap map[string]interface{}, gatewayURL string, agentPath string) map[string]interface{} {
	externalURL := constructExternalURL(gatewayURL, agentPath)

	// Rewrite main URL
	if _, ok := safeGetString(cardMap, "url"); ok {
		cardMap["url"] = externalURL
	}

	// Rewrite and filter additional interfaces
	if interfaces, ok := safeGetArray(cardMap, "additionalInterfaces"); ok {
		cardMap["additionalInterfaces"] = rewriteAdditionalInterfacesMap(interfaces, gatewayURL, agentPath)
	}

	// Provider URL is never rewritten (it's organizational metadata, not an agent endpoint)

	return cardMap
}
