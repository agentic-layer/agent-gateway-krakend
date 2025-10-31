package main

import (
	"testing"

	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
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

func TestRewriteAdditionalInterfaces(t *testing.T) {
	tests := []struct {
		name       string
		interfaces []models.AgentInterface
		gatewayURL string
		agentPath  string
		expected   []models.AgentInterface
	}{
		{
			name: "rewrite internal http and https, remove grpc",
			interfaces: []models.AgentInterface{
				{Transport: "http", Url: "http://agent.svc.cluster.local:8000/"},
				{Transport: "https", Url: "https://agent.svc.cluster.local:8443/"},
				{Transport: "grpc", Url: "http://agent.svc.cluster.local:9000/"},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			expected: []models.AgentInterface{
				{Transport: "http", Url: "https://gateway.ai/test-agent"},
				{Transport: "https", Url: "https://gateway.ai/test-agent"},
			},
		},
		{
			name: "rewrite all URLs",
			interfaces: []models.AgentInterface{
				{Transport: "http", Url: "https://external.example.com/agent"},
				{Transport: "http", Url: "http://agent.svc.cluster.local:8000/"},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			expected: []models.AgentInterface{
				{Transport: "http", Url: "https://gateway.ai/test-agent"},
				{Transport: "http", Url: "https://gateway.ai/test-agent"},
			},
		},
		{
			name: "remove all non-http transports",
			interfaces: []models.AgentInterface{
				{Transport: "grpc", Url: "http://agent.svc.cluster.local:9000/"},
				{Transport: "websocket", Url: "ws://agent.svc.cluster.local:8080/"},
				{Transport: "sse", Url: "http://agent.svc.cluster.local:8000/events"},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			expected:   []models.AgentInterface{},
		},
		{
			name:       "empty interfaces",
			interfaces: []models.AgentInterface{},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			expected:   []models.AgentInterface{},
		},
		{
			name: "only keep http and https",
			interfaces: []models.AgentInterface{
				{Transport: "http", Url: "http://agent.svc.cluster.local:8000/"},
				{Transport: "JSONRPC", Url: "http://agent.svc.cluster.local:8000/"},
				{Transport: "HTTP+JSON", Url: "http://agent.svc.cluster.local:8000/"},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			expected: []models.AgentInterface{
				{Transport: "http", Url: "https://gateway.ai/test-agent"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewriteAdditionalInterfaces(tt.interfaces, tt.gatewayURL, tt.agentPath)

			if len(result) != len(tt.expected) {
				t.Errorf("rewriteAdditionalInterfaces() returned %d interfaces, want %d",
					len(result), len(tt.expected))
				return
			}

			for i, iface := range result {
				if iface.Transport != tt.expected[i].Transport {
					t.Errorf("interface[%d].Transport = %q, want %q",
						i, iface.Transport, tt.expected[i].Transport)
				}
				if iface.Url != tt.expected[i].Url {
					t.Errorf("interface[%d].Url = %q, want %q",
						i, iface.Url, tt.expected[i].Url)
				}
			}
		})
	}
}

func TestRewriteAgentCard(t *testing.T) {
	tests := []struct {
		name       string
		card       models.AgentCard
		gatewayURL string
		agentPath  string
		checkFunc  func(t *testing.T, result models.AgentCard)
	}{
		{
			name: "rewrite internal main URL",
			card: models.AgentCard{
				Name:        "Test Agent",
				Description: "A test agent",
				Url:         "http://agent.default.svc.cluster.local:8000/",
				Version:     "1.0.0",
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			checkFunc: func(t *testing.T, result models.AgentCard) {
				if result.Url != "https://gateway.ai/test-agent" {
					t.Errorf("card.Url = %q, want %q", result.Url, "https://gateway.ai/test-agent")
				}
				if result.Name != "Test Agent" {
					t.Errorf("card.Name changed unexpectedly")
				}
			},
		},
		{
			name: "rewrite external main URL",
			card: models.AgentCard{
				Url:     "https://external.example.com/agent",
				Version: "1.0.0",
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			checkFunc: func(t *testing.T, result models.AgentCard) {
				if result.Url != "https://gateway.ai/test-agent" {
					t.Errorf("card.Url = %q, want %q", result.Url, "https://gateway.ai/test-agent")
				}
			},
		},
		{
			name: "rewrite additionalInterfaces",
			card: models.AgentCard{
				Url:     "http://agent.svc.cluster.local:8000/",
				Version: "1.0.0",
				AdditionalInterfaces: []models.AgentInterface{
					{Transport: "http", Url: "http://agent.svc.cluster.local:8000/"},
					{Transport: "grpc", Url: "http://agent.svc.cluster.local:9000/"},
				},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			checkFunc: func(t *testing.T, result models.AgentCard) {
				if len(result.AdditionalInterfaces) != 1 {
					t.Errorf("len(AdditionalInterfaces) = %d, want 1", len(result.AdditionalInterfaces))
					return
				}
				if result.AdditionalInterfaces[0].Transport != "http" {
					t.Errorf("AdditionalInterfaces[0].Transport = %q, want http", result.AdditionalInterfaces[0].Transport)
				}
				if result.AdditionalInterfaces[0].Url != "https://gateway.ai/test-agent" {
					t.Errorf("AdditionalInterfaces[0].Url = %q, want https://gateway.ai/test-agent", result.AdditionalInterfaces[0].Url)
				}
			},
		},
		{
			name: "never rewrite provider URL",
			card: models.AgentCard{
				Url:     "http://agent.svc.cluster.local:8000/",
				Version: "1.0.0",
				Provider: &models.AgentProvider{
					Organization: "Test Org",
					Url:          "https://qaware.de",
				},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			checkFunc: func(t *testing.T, result models.AgentCard) {
				if result.Provider == nil {
					t.Errorf("Provider is nil")
					return
				}
				if result.Provider.Url != "https://qaware.de" {
					t.Errorf("Provider.Url = %q, want unchanged", result.Provider.Url)
				}
			},
		},
		{
			name: "preserve all other fields",
			card: models.AgentCard{
				Name:               "Test Agent",
				Description:        "A test description",
				Url:                "http://agent.svc.cluster.local:8000/",
				Version:            "1.0.0",
				ProtocolVersion:    "0.3.0",
				DefaultInputModes:  []string{"text/plain"},
				DefaultOutputModes: []string{"text/plain"},
				Skills:             []models.AgentSkill{{Id: "test-skill", Name: "Test Skill", Tags: []string{"test"}}},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			checkFunc: func(t *testing.T, result models.AgentCard) {
				if result.Name != "Test Agent" {
					t.Errorf("Name changed")
				}
				if result.Description != "A test description" {
					t.Errorf("Description changed")
				}
				if result.Version != "1.0.0" {
					t.Errorf("Version changed")
				}
				if result.ProtocolVersion != "0.3.0" {
					t.Errorf("ProtocolVersion changed")
				}
				if len(result.Skills) != 1 || result.Skills[0].Id != "test-skill" {
					t.Errorf("Skills changed")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewriteAgentCard(tt.card, tt.gatewayURL, tt.agentPath)
			tt.checkFunc(t, result)
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
			name: "rewrite internal http and https, remove grpc",
			interfaces: []interface{}{
				map[string]interface{}{"transport": "http", "url": "http://agent.svc.cluster.local:8000/"},
				map[string]interface{}{"transport": "https", "url": "https://agent.svc.cluster.local:8443/"},
				map[string]interface{}{"transport": "grpc", "url": "http://agent.svc.cluster.local:9000/"},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			expected: []interface{}{
				map[string]interface{}{"transport": "http", "url": "https://gateway.ai/test-agent"},
				map[string]interface{}{"transport": "https", "url": "https://gateway.ai/test-agent"},
			},
		},
		{
			name: "preserve custom fields in interfaces",
			interfaces: []interface{}{
				map[string]interface{}{
					"transport":   "http",
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
					"transport":   "http",
					"url":         "https://gateway.ai/test-agent",
					"customField": "custom-value",
					"nested": map[string]interface{}{
						"key": "value",
					},
				},
			},
		},
		{
			name: "rewrite all URLs",
			interfaces: []interface{}{
				map[string]interface{}{"transport": "http", "url": "https://external.example.com/agent"},
				map[string]interface{}{"transport": "http", "url": "http://agent.svc.cluster.local:8000/"},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			expected: []interface{}{
				map[string]interface{}{"transport": "http", "url": "https://gateway.ai/test-agent"},
				map[string]interface{}{"transport": "http", "url": "https://gateway.ai/test-agent"},
			},
		},
		{
			name: "remove all non-http transports",
			interfaces: []interface{}{
				map[string]interface{}{"transport": "grpc", "url": "http://agent.svc.cluster.local:9000/"},
				map[string]interface{}{"transport": "websocket", "url": "ws://agent.svc.cluster.local:8080/"},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			expected:   []interface{}{},
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
			name: "rewrite additionalInterfaces",
			cardMap: map[string]interface{}{
				"url":     "http://agent.svc.cluster.local:8000/",
				"version": "1.0.0",
				"additionalInterfaces": []interface{}{
					map[string]interface{}{"transport": "http", "url": "http://agent.svc.cluster.local:8000/"},
					map[string]interface{}{"transport": "grpc", "url": "http://agent.svc.cluster.local:9000/"},
				},
			},
			gatewayURL: "https://gateway.ai",
			agentPath:  "/test-agent",
			checkFunc: func(t *testing.T, result map[string]interface{}) {
				interfaces := result["additionalInterfaces"].([]interface{})
				if len(interfaces) != 1 {
					t.Errorf("len(additionalInterfaces) = %d, want 1", len(interfaces))
					return
				}
				iface := interfaces[0].(map[string]interface{})
				if iface["transport"] != "http" {
					t.Errorf("interface transport = %v, want http", iface["transport"])
				}
				if iface["url"] != "https://gateway.ai/test-agent" {
					t.Errorf("interface url = %v, want rewritten URL", iface["url"])
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
