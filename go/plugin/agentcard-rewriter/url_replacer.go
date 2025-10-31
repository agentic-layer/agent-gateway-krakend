package main

import (
	"strings"
)

// Valid transport protocol constants
const (
	transportJSONRPC  = "jsonrpc"
	transportGRPC     = "grpc"
	transportHTTPJSON = "http+json"
)

// isValidTransport checks if a transport type is valid (case-insensitive)
func isValidTransport(transport string) bool {
	normalized := strings.ToLower(transport)
	return normalized == transportJSONRPC ||
		normalized == transportGRPC ||
		normalized == transportHTTPJSON
}

// constructExternalURL builds the external gateway URL from gateway URL and agent path
func constructExternalURL(gatewayURL string, agentPath string) string {
	// Remove trailing slashes
	cleanGatewayURL := strings.TrimSuffix(gatewayURL, "/")
	cleanAgentPath := strings.TrimSuffix(agentPath, "/")

	return cleanGatewayURL + cleanAgentPath
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

// rewriteAdditionalInterfacesMap filters and rewrites additional interfaces using map representation
// - Keeps only valid transports (JSONRPC, GRPC, HTTP+JSON) - case-insensitive
// - Rewrites URLs to external gateway URLs
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

		// Only keep valid transports (JSONRPC, GRPC, HTTP+JSON - case-insensitive)
		if isValidTransport(transport) {
			// Rewrite URLs to gateway URL
			if _, ok := safeGetString(ifaceMap, "url"); ok {
				ifaceMap["url"] = externalURL
			}
			result = append(result, ifaceMap)
		}
		// All invalid/unknown transports are implicitly removed
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
	return cardMap
}
