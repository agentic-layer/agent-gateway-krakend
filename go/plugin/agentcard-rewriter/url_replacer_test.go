package main

import (
	"testing"

	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
)

func TestConstructExternalURL(t *testing.T) {
	tests := []struct {
		name          string
		gatewayDomain string
		agentName     string
		pathPrefix    string
		expected      string
	}{
		{
			name:          "basic construction without prefix",
			gatewayDomain: "https://gateway.agentic-layer.ai",
			agentName:     "weather-agent",
			pathPrefix:    "",
			expected:      "https://gateway.agentic-layer.ai/weather-agent",
		},
		{
			name:          "gateway domain with trailing slash",
			gatewayDomain: "https://gateway.agentic-layer.ai/",
			agentName:     "agent",
			pathPrefix:    "",
			expected:      "https://gateway.agentic-layer.ai/agent",
		},
		{
			name:          "localhost without prefix",
			gatewayDomain: "http://localhost:10000",
			agentName:     "test-agent",
			pathPrefix:    "",
			expected:      "http://localhost:10000/test-agent",
		},
		{
			name:          "agent with hyphens",
			gatewayDomain: "https://gateway.ai",
			agentName:     "cross-selling-agent",
			pathPrefix:    "",
			expected:      "https://gateway.ai/cross-selling-agent",
		},
		{
			name:          "with /agents prefix",
			gatewayDomain: "https://gateway.agentic-layer.ai",
			agentName:     "weather-agent",
			pathPrefix:    "/agents",
			expected:      "https://gateway.agentic-layer.ai/agents/weather-agent",
		},
		{
			name:          "with /api/v1/agents prefix",
			gatewayDomain: "https://gateway.ai",
			agentName:     "test-agent",
			pathPrefix:    "/api/v1/agents",
			expected:      "https://gateway.ai/api/v1/agents/test-agent",
		},
		{
			name:          "prefix with trailing slash",
			gatewayDomain: "https://gateway.ai",
			agentName:     "test-agent",
			pathPrefix:    "/agents/",
			expected:      "https://gateway.ai/agents/test-agent",
		},
		{
			name:          "both trailing slashes",
			gatewayDomain: "https://gateway.ai/",
			agentName:     "test-agent",
			pathPrefix:    "/agents/",
			expected:      "https://gateway.ai/agents/test-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := constructExternalURL(tt.gatewayDomain, tt.agentName, tt.pathPrefix)
			if result != tt.expected {
				t.Errorf("constructExternalURL(%q, %q, %q) = %q, want %q",
					tt.gatewayDomain, tt.agentName, tt.pathPrefix, result, tt.expected)
			}
		})
	}
}

func TestCheckProviderURL(t *testing.T) {
	tests := []struct {
		name        string
		providerURL string
		shouldWarn  bool
	}{
		{
			name:        "internal cluster URL",
			providerURL: "http://company.svc.cluster.local",
			shouldWarn:  true,
		},
		{
			name:        "internal cluster URL with port",
			providerURL: "http://service.namespace.svc.cluster.local:8080",
			shouldWarn:  true,
		},
		{
			name:        "external URL",
			providerURL: "https://qaware.de",
			shouldWarn:  false,
		},
		{
			name:        "external company URL",
			providerURL: "https://example.com",
			shouldWarn:  false,
		},
		{
			name:        "localhost",
			providerURL: "http://localhost",
			shouldWarn:  false,
		},
		{
			name:        "empty URL",
			providerURL: "",
			shouldWarn:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldWarn, reason := checkProviderURL(tt.providerURL)
			if shouldWarn != tt.shouldWarn {
				t.Errorf("checkProviderURL(%q) shouldWarn = %v, want %v (reason: %s)",
					tt.providerURL, shouldWarn, tt.shouldWarn, reason)
			}
			if shouldWarn && reason == "" {
				t.Errorf("checkProviderURL(%q) shouldWarn=true but reason is empty", tt.providerURL)
			}
		})
	}
}

func TestRewriteAdditionalInterfaces(t *testing.T) {
	tests := []struct {
		name          string
		interfaces    []models.AgentInterface
		gatewayDomain string
		agentName     string
		expected      []models.AgentInterface
	}{
		{
			name: "rewrite internal http and https, remove grpc",
			interfaces: []models.AgentInterface{
				{Transport: "http", Url: "http://agent.svc.cluster.local:8000/"},
				{Transport: "https", Url: "https://agent.svc.cluster.local:8443/"},
				{Transport: "grpc", Url: "http://agent.svc.cluster.local:9000/"},
			},
			gatewayDomain: "https://gateway.ai",
			agentName:     "test-agent",
			expected: []models.AgentInterface{
				{Transport: "http", Url: "https://gateway.ai/test-agent"},
				{Transport: "https", Url: "https://gateway.ai/test-agent"},
			},
		},
		{
			name: "keep external http, rewrite internal http",
			interfaces: []models.AgentInterface{
				{Transport: "http", Url: "https://external.example.com/agent"},
				{Transport: "http", Url: "http://agent.svc.cluster.local:8000/"},
			},
			gatewayDomain: "https://gateway.ai",
			agentName:     "test-agent",
			expected: []models.AgentInterface{
				{Transport: "http", Url: "https://external.example.com/agent"},
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
			gatewayDomain: "https://gateway.ai",
			agentName:     "test-agent",
			expected:      []models.AgentInterface{},
		},
		{
			name:          "empty interfaces",
			interfaces:    []models.AgentInterface{},
			gatewayDomain: "https://gateway.ai",
			agentName:     "test-agent",
			expected:      []models.AgentInterface{},
		},
		{
			name: "only keep http and https",
			interfaces: []models.AgentInterface{
				{Transport: "http", Url: "http://agent.svc.cluster.local:8000/"},
				{Transport: "JSONRPC", Url: "http://agent.svc.cluster.local:8000/"},
				{Transport: "HTTP+JSON", Url: "http://agent.svc.cluster.local:8000/"},
			},
			gatewayDomain: "https://gateway.ai",
			agentName:     "test-agent",
			expected: []models.AgentInterface{
				{Transport: "http", Url: "https://gateway.ai/test-agent"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewriteAdditionalInterfaces(tt.interfaces, tt.gatewayDomain, tt.agentName, "")

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
		name          string
		card          models.AgentCard
		gatewayDomain string
		agentName     string
		checkFunc     func(t *testing.T, result models.AgentCard)
	}{
		{
			name: "rewrite internal main URL",
			card: models.AgentCard{
				Name:        "Test Agent",
				Description: "A test agent",
				Url:         "http://agent.default.svc.cluster.local:8000/",
				Version:     "1.0.0",
			},
			gatewayDomain: "https://gateway.ai",
			agentName:     "test-agent",
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
			name: "keep external main URL unchanged",
			card: models.AgentCard{
				Url:     "https://external.example.com/agent",
				Version: "1.0.0",
			},
			gatewayDomain: "https://gateway.ai",
			agentName:     "test-agent",
			checkFunc: func(t *testing.T, result models.AgentCard) {
				if result.Url != "https://external.example.com/agent" {
					t.Errorf("card.Url = %q, want unchanged", result.Url)
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
			gatewayDomain: "https://gateway.ai",
			agentName:     "test-agent",
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
			gatewayDomain: "https://gateway.ai",
			agentName:     "test-agent",
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
			gatewayDomain: "https://gateway.ai",
			agentName:     "test-agent",
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
			result := rewriteAgentCard(tt.card, tt.gatewayDomain, tt.agentName, "")
			tt.checkFunc(t, result)
		})
	}
}
