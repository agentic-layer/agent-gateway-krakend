package main

import (
	"testing"
)

func TestConstructExternalURL(t *testing.T) {
	tests := []struct {
		name       string
		gatewayURL string
		agentPath  string
		expected   string
	}{
		{
			name:       "basic construction",
			gatewayURL: "https://gateway.agentic-layer.ai",
			agentPath:  "/weather-agent",
			expected:   "https://gateway.agentic-layer.ai/weather-agent",
		},
		{
			name:       "gateway URL with trailing slash",
			gatewayURL: "https://gateway.agentic-layer.ai/",
			agentPath:  "/agent",
			expected:   "https://gateway.agentic-layer.ai/agent",
		},
		{
			name:       "localhost",
			gatewayURL: "http://localhost:10000",
			agentPath:  "/test-agent",
			expected:   "http://localhost:10000/test-agent",
		},
		{
			name:       "agent with hyphens",
			gatewayURL: "https://gateway.ai",
			agentPath:  "/cross-selling-agent",
			expected:   "https://gateway.ai/cross-selling-agent",
		},
		{
			name:       "nested path with multiple segments",
			gatewayURL: "https://gateway.agentic-layer.ai",
			agentPath:  "/agents/weather-agent",
			expected:   "https://gateway.agentic-layer.ai/agents/weather-agent",
		},
		{
			name:       "deeply nested path",
			gatewayURL: "https://gateway.ai",
			agentPath:  "/api/v1/agents/test-agent",
			expected:   "https://gateway.ai/api/v1/agents/test-agent",
		},
		{
			name:       "agent path with trailing slash",
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent/",
			expected:   "https://gateway.ai/test-agent",
		},
		{
			name:       "both trailing slashes",
			gatewayURL: "https://gateway.ai/",
			agentPath:  "/agents/test-agent/",
			expected:   "https://gateway.ai/agents/test-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := constructExternalURL(tt.gatewayURL, tt.agentPath)
			if result != tt.expected {
				t.Errorf("constructExternalURL(%q, %q) = %q, want %q",
					tt.gatewayURL, tt.agentPath, result, tt.expected)
			}
		})
	}
}

// TestRewriteAdditionalInterfacesMap tests the map-based additional interfaces rewrite function
func TestRewriteAdditionalInterfacesMap(t *testing.T) {
	tests := []struct {
		name       string
		interfaces []interface{}
		gatewayURL string
		agentPath  string
		expected   []interface{}
	}{
		{
			name: "rewrite all valid transports (JSONRPC, GRPC, HTTP+JSON)",
			interfaces: []interface{}{
				map[string]interface{}{"transport": "JSONRPC", "url": "http://agent.svc.cluster.local:8000/"},
				map[string]interface{}{"transport": "GRPC", "url": "https://agent.svc.cluster.local:8443/"},
				map[string]interface{}{"transport": "HTTP+JSON", "url": "http://agent.svc.cluster.local:9000/"},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			expected: []interface{}{
				map[string]interface{}{"transport": "JSONRPC", "url": "https://gateway.ai/test-agent"},
				map[string]interface{}{"transport": "GRPC", "url": "https://gateway.ai/test-agent"},
				map[string]interface{}{"transport": "HTTP+JSON", "url": "https://gateway.ai/test-agent"},
			},
		},
		{
			name: "preserve custom fields in interfaces",
			interfaces: []interface{}{
				map[string]interface{}{
					"transport":   "HTTP+JSON",
					"url":         "http://agent.svc.cluster.local:8000/",
					"customField": "custom-value",
					"nested": map[string]interface{}{
						"key": "value",
					},
				},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			expected: []interface{}{
				map[string]interface{}{
					"transport":   "HTTP+JSON",
					"url":         "https://gateway.ai/test-agent",
					"customField": "custom-value",
					"nested": map[string]interface{}{
						"key": "value",
					},
				},
			},
		},
		{
			name: "rewrite all URLs with valid transports",
			interfaces: []interface{}{
				map[string]interface{}{"transport": "JSONRPC", "url": "https://external.example.com/agent"},
				map[string]interface{}{"transport": "HTTP+JSON", "url": "http://agent.svc.cluster.local:8000/"},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			expected: []interface{}{
				map[string]interface{}{"transport": "JSONRPC", "url": "https://gateway.ai/test-agent"},
				map[string]interface{}{"transport": "HTTP+JSON", "url": "https://gateway.ai/test-agent"},
			},
		},
		{
			name: "filter invalid transports, keep valid ones",
			interfaces: []interface{}{
				map[string]interface{}{"transport": "grpc", "url": "http://agent.svc.cluster.local:9000/"},
				map[string]interface{}{"transport": "websocket", "url": "ws://agent.svc.cluster.local:8080/"},
				map[string]interface{}{"transport": "http", "url": "http://agent.svc.cluster.local:8000/"},
				map[string]interface{}{"transport": "https", "url": "https://agent.svc.cluster.local:8443/"},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			expected: []interface{}{
				map[string]interface{}{"transport": "grpc", "url": "https://gateway.ai/test-agent"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewriteAdditionalInterfacesMap(tt.interfaces, tt.gatewayURL, tt.agentPath)

			if len(result) != len(tt.expected) {
				t.Errorf("rewriteAdditionalInterfacesMap() returned %d interfaces, want %d",
					len(result), len(tt.expected))
				return
			}

			for i, iface := range result {
				ifaceMap := iface.(map[string]interface{})
				expectedMap := tt.expected[i].(map[string]interface{})

				for key, expectedVal := range expectedMap {
					if actualVal, ok := ifaceMap[key]; !ok {
						t.Errorf("interface[%d] missing key %q", i, key)
					} else {
						// Deep comparison for nested maps
						if nestedExpected, ok := expectedVal.(map[string]interface{}); ok {
							if nestedActual, ok := actualVal.(map[string]interface{}); ok {
								for nestedKey, nestedExpectedVal := range nestedExpected {
									if nestedActualVal := nestedActual[nestedKey]; nestedActualVal != nestedExpectedVal {
										t.Errorf("interface[%d].%s.%s = %v, want %v",
											i, key, nestedKey, nestedActualVal, nestedExpectedVal)
									}
								}
							}
						} else if actualVal != expectedVal {
							t.Errorf("interface[%d].%s = %v, want %v", i, key, actualVal, expectedVal)
						}
					}
				}
			}
		})
	}
}

// TestRewriteAgentCardMap tests the map-based agent card rewrite function
func TestRewriteAgentCardMap(t *testing.T) {
	tests := []struct {
		name       string
		cardMap    map[string]interface{}
		gatewayURL string
		agentPath  string
		checkFunc  func(t *testing.T, result map[string]interface{})
	}{
		{
			name: "rewrite internal main URL",
			cardMap: map[string]interface{}{
				"name":        "Test Agent",
				"description": "A test agent",
				"url":         "http://agent.default.svc.cluster.local:8000/",
				"version":     "1.0.0",
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			checkFunc: func(t *testing.T, result map[string]interface{}) {
				if result["url"] != "https://gateway.ai/test-agent" {
					t.Errorf("url = %v, want %q", result["url"], "https://gateway.ai/test-agent")
				}
				if result["name"] != "Test Agent" {
					t.Error("name changed unexpectedly")
				}
			},
		},
		{
			name: "preserve unknown fields",
			cardMap: map[string]interface{}{
				"url":     "http://agent.svc.cluster.local:8000/",
				"version": "1.0.0",
				"x-custom-metadata": map[string]interface{}{
					"vendor": "ACME",
				},
				"experimental-feature": "enabled",
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			checkFunc: func(t *testing.T, result map[string]interface{}) {
				if _, ok := result["x-custom-metadata"]; !ok {
					t.Error("x-custom-metadata field was lost")
				}
				if result["experimental-feature"] != "enabled" {
					t.Error("experimental-feature field was lost or changed")
				}
			},
		},
		{
			name: "rewrite external main URL",
			cardMap: map[string]interface{}{
				"url":     "https://external.example.com/agent",
				"version": "1.0.0",
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			checkFunc: func(t *testing.T, result map[string]interface{}) {
				if result["url"] != "https://gateway.ai/test-agent" {
					t.Errorf("url = %v, want %q", result["url"], "https://gateway.ai/test-agent")
				}
			},
		},
		{
			name: "rewrite and filter additionalInterfaces",
			cardMap: map[string]interface{}{
				"url":     "http://agent.svc.cluster.local:8000/",
				"version": "1.0.0",
				"additionalInterfaces": []interface{}{
					map[string]interface{}{"transport": "HTTP+JSON", "url": "http://agent.svc.cluster.local:8000/"},
					map[string]interface{}{"transport": "grpc", "url": "http://agent.svc.cluster.local:9000/"},
					map[string]interface{}{"transport": "websocket", "url": "ws://agent.svc.cluster.local:8080/"},
				},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			checkFunc: func(t *testing.T, result map[string]interface{}) {
				interfaces := result["additionalInterfaces"].([]interface{})
				if len(interfaces) != 2 {
					t.Errorf("len(additionalInterfaces) = %d, want 2 (HTTP+JSON and grpc are valid, websocket filtered)", len(interfaces))
					return
				}
				// First interface should be HTTP+JSON
				iface0 := interfaces[0].(map[string]interface{})
				if iface0["transport"] != "HTTP+JSON" {
					t.Errorf("interface[0] transport = %v, want HTTP+JSON", iface0["transport"])
				}
				if iface0["url"] != "https://gateway.ai/test-agent" {
					t.Errorf("interface[0] url = %v, want rewritten URL", iface0["url"])
				}
				// Second interface should be grpc
				iface1 := interfaces[1].(map[string]interface{})
				if iface1["transport"] != "grpc" {
					t.Errorf("interface[1] transport = %v, want grpc", iface1["transport"])
				}
				if iface1["url"] != "https://gateway.ai/test-agent" {
					t.Errorf("interface[1] url = %v, want rewritten URL", iface1["url"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewriteAgentCardMap(tt.cardMap, tt.gatewayURL, tt.agentPath)
			tt.checkFunc(t, result)
		})
	}
}
