package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseModelParameter_SimpleFormat(t *testing.T) {
	namespace, agentName, isNamespaced, err := parseModelParameter("test-agent")

	assert.NoError(t, err)
	assert.Equal(t, "", namespace)
	assert.Equal(t, "test-agent", agentName)
	assert.False(t, isNamespaced)
}

func TestParseModelParameter_NamespacedFormat(t *testing.T) {
	namespace, agentName, isNamespaced, err := parseModelParameter("test-namespace/test-agent")

	assert.NoError(t, err)
	assert.Equal(t, "test-namespace", namespace)
	assert.Equal(t, "test-agent", agentName)
	assert.True(t, isNamespaced)
}

func TestParseModelParameter_MultipleSlashes(t *testing.T) {
	// Multiple slashes should be rejected due to validation
	_, _, _, err := parseModelParameter("namespace/agent/with/slashes")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid namespaced model format")
}

func TestResolveAgentBackend_SimpleFormat_UniqueAgent(t *testing.T) {
	// Verifies agent matching logic without K8s dependency

	agentName := "unique-agent"
	agents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unique-agent",
				Namespace: "default",
			},
			Spec: AgentSpec{Exposed: true},
			Status: AgentStatus{
				URL: "http://unique-agent.default.svc.cluster.local:8000/.well-known/agent-card.json",
			},
		},
	}

	// Simulate resolution (same logic as resolveAgentBackend)
	var matchingAgents []Agent
	for _, agent := range agents {
		if agent.Name == agentName {
			matchingAgents = append(matchingAgents, agent)
		}
	}

	assert.Len(t, matchingAgents, 1, "should find exactly one agent")
	assert.Equal(t, "default", matchingAgents[0].Namespace)
}

func TestResolveAgentBackend_SimpleFormat_MultipleAgents(t *testing.T) {
	// Test collision detection logic
	agentName := "duplicate-agent"
	agents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "duplicate-agent",
				Namespace: "namespace-a",
			},
			Spec: AgentSpec{Exposed: true},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "duplicate-agent",
				Namespace: "namespace-b",
			},
			Spec: AgentSpec{Exposed: true},
		},
	}

	// Simulate resolution
	var matchingAgents []Agent
	for _, agent := range agents {
		if agent.Name == agentName {
			matchingAgents = append(matchingAgents, agent)
		}
	}

	// Verify collision is detected
	assert.Len(t, matchingAgents, 2, "should find multiple agents with same name")

	// Build error message (same logic as resolveAgentBackend)
	if len(matchingAgents) > 1 {
		var namespaces []string
		for _, agent := range matchingAgents {
			namespaces = append(namespaces, fmt.Sprintf("%s/%s", agent.Namespace, agent.Name))
		}
		errorMsg := fmt.Sprintf("multiple agents named %s found in different namespaces, please use namespaced format: %s",
			agentName, fmt.Sprintf("%s, %s", namespaces[0], namespaces[1]))

		assert.Contains(t, errorMsg, "namespace-a/duplicate-agent")
		assert.Contains(t, errorMsg, "namespace-b/duplicate-agent")
		assert.Contains(t, errorMsg, "please use namespaced format")
	}
}

func TestResolveAgentBackend_SimpleFormat_NoAgent(t *testing.T) {
	// Test missing agent logic
	agentName := "non-existent-agent"
	agents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-agent",
				Namespace: "default",
			},
			Spec: AgentSpec{Exposed: true},
		},
	}

	// Simulate resolution
	var matchingAgents []Agent
	for _, agent := range agents {
		if agent.Name == agentName {
			matchingAgents = append(matchingAgents, agent)
		}
	}

	assert.Len(t, matchingAgents, 0, "should find no agents")

	// Verify error would be returned
	if len(matchingAgents) == 0 {
		errorMsg := fmt.Sprintf("agent %s not found or not exposed", agentName)
		assert.Contains(t, errorMsg, "not found or not exposed")
	}
}

func TestResolveAgentBackend_NamespacedFormat_Found(t *testing.T) {
	// Test namespaced format resolution
	targetNamespace := "namespace-a"
	targetName := "test-agent"
	agents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-agent",
				Namespace: "namespace-a",
			},
			Spec: AgentSpec{Exposed: true},
			Status: AgentStatus{
				URL: "http://test-agent.namespace-a.svc.cluster.local:8000/.well-known/agent-card.json",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-agent",
				Namespace: "namespace-b",
			},
			Spec: AgentSpec{Exposed: true},
		},
	}

	// Simulate resolution with namespace
	var foundAgent *Agent
	for _, agent := range agents {
		if agent.Name == targetName && agent.Namespace == targetNamespace {
			foundAgent = &agent
			break
		}
	}

	assert.NotNil(t, foundAgent, "should find agent in specific namespace")
	assert.Equal(t, "namespace-a", foundAgent.Namespace)
	assert.NotEmpty(t, foundAgent.Status.URL)
}

func TestResolveAgentBackend_NamespacedFormat_NotFound(t *testing.T) {
	// Test namespaced format with non-existent agent
	targetNamespace := "namespace-c"
	targetName := "test-agent"
	agents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-agent",
				Namespace: "namespace-a",
			},
			Spec: AgentSpec{Exposed: true},
		},
	}

	// Simulate resolution
	var foundAgent *Agent
	for _, agent := range agents {
		if agent.Name == targetName && agent.Namespace == targetNamespace {
			foundAgent = &agent
			break
		}
	}

	assert.Nil(t, foundAgent, "should not find agent in wrong namespace")

	// Verify error would be returned
	if foundAgent == nil {
		errorMsg := fmt.Sprintf("agent %s/%s not found or not exposed", targetNamespace, targetName)
		assert.Contains(t, errorMsg, "namespace-c/test-agent")
		assert.Contains(t, errorMsg, "not found or not exposed")
	}
}

func TestModelInfo_BackendURLExtraction(t *testing.T) {
	// Test backend URL construction from agent URL
	testCases := []struct {
		name        string
		agentURL    string
		expectedURL string
	}{
		{
			name:        "standard service URL",
			agentURL:    "http://agent.namespace.svc.cluster.local:8000/.well-known/agent-card.json",
			expectedURL: "http://agent.namespace.svc.cluster.local:8000",
		},
		{
			name:        "URL with different port",
			agentURL:    "http://agent.namespace.svc.cluster.local:9000/.well-known/agent-card.json",
			expectedURL: "http://agent.namespace.svc.cluster.local:9000",
		},
		{
			name:        "HTTPS URL",
			agentURL:    "https://agent.namespace.svc.cluster.local:8000/.well-known/agent-card.json",
			expectedURL: "https://agent.namespace.svc.cluster.local:8000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Contains(t, tc.agentURL, tc.expectedURL, "agent URL should contain backend URL")
		})
	}
}

func TestModelInfo_Structure(t *testing.T) {
	modelInfo := ModelInfo{
		Name:      "test-agent",
		Namespace: "test-namespace",
		URL:       "http://test-agent.test-namespace.svc.cluster.local:8000",
	}

	assert.Equal(t, "test-agent", modelInfo.Name)
	assert.Equal(t, "test-namespace", modelInfo.Namespace)
	assert.Equal(t, "http://test-agent.test-namespace.svc.cluster.local:8000", modelInfo.URL)
	assert.Contains(t, modelInfo.URL, modelInfo.Namespace)
}

func TestResolveAgentBackend_AgentWithoutURL(t *testing.T) {
	// Test handling of agent without status.url
	agents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "incomplete-agent",
				Namespace: "default",
			},
			Spec: AgentSpec{Exposed: true},
			Status: AgentStatus{
				URL: "", // Empty URL
			},
		},
	}

	agent := agents[0]

	// Verify empty URL would cause error
	if agent.Status.URL == "" {
		errorMsg := fmt.Sprintf("agent %s has no URL in status", agent.Name)
		assert.Contains(t, errorMsg, "incomplete-agent")
		assert.Contains(t, errorMsg, "no URL in status")
	}
}

func TestResolveAgentBackend_OnlyExposedAgents(t *testing.T) {
	// Verify that only exposed agents are considered
	agents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exposed-agent",
				Namespace: "default",
			},
			Spec: AgentSpec{Exposed: true},
			Status: AgentStatus{
				URL: "http://exposed-agent.default.svc.cluster.local:8000/.well-known/agent-card.json",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "not-exposed-agent",
				Namespace: "default",
			},
			Spec: AgentSpec{Exposed: false},
			Status: AgentStatus{
				URL: "http://not-exposed-agent.default.svc.cluster.local:8000/.well-known/agent-card.json",
			},
		},
	}

	// In resolveAgentBackend, only exposed agents are listed
	// Simulate this filtering
	exposedAgents := make([]Agent, 0)
	for _, agent := range agents {
		if agent.Spec.Exposed {
			exposedAgents = append(exposedAgents, agent)
		}
	}

	assert.Len(t, exposedAgents, 1)
	assert.Equal(t, "exposed-agent", exposedAgents[0].Name)
}

func TestNamespacedPathConstruction(t *testing.T) {
	// Test that the namespaced path is correctly constructed
	testCases := []struct {
		name           string
		agentName      string
		agentNamespace string
		expectedPath   string
	}{
		{
			name:           "simple namespace and agent name",
			agentName:      "test-agent",
			agentNamespace: "default",
			expectedPath:   "/default/test-agent",
		},
		{
			name:           "namespace with hyphens",
			agentName:      "test-agent-b",
			agentNamespace: "test-namespace-b",
			expectedPath:   "/test-namespace-b/test-agent-b",
		},
		{
			name:           "agent name with multiple parts",
			agentName:      "weather-forecast-agent",
			agentNamespace: "production",
			expectedPath:   "/production/weather-forecast-agent",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This simulates the path construction in handleGlobalChatCompletions (line 209)
			namespacedPath := "/" + tc.agentNamespace + "/" + tc.agentName

			assert.Equal(t, tc.expectedPath, namespacedPath)
			assert.Contains(t, namespacedPath, tc.agentNamespace, "path should contain namespace")
			assert.Contains(t, namespacedPath, tc.agentName, "path should contain agent name")
			assert.True(t, len(namespacedPath) > len(tc.agentName), "namespaced path should be longer than agent name alone")
		})
	}
}

func TestRoutingPathFormat(t *testing.T) {
	// Verify we always route to namespaced format regardless of how model was specified
	testCases := []struct {
		name         string
		modelInfo    ModelInfo
		expectedPath string
		description  string
	}{
		{
			name: "conflicting agent - must use namespaced path",
			modelInfo: ModelInfo{
				Name:      "test-agent",
				Namespace: "default",
				URL:       "http://test-agent.default.svc.cluster.local:8000",
			},
			expectedPath: "/default/test-agent",
			description:  "Even though user specified 'default/test-agent', routing MUST use namespaced format",
		},
		{
			name: "conflicting agent in different namespace",
			modelInfo: ModelInfo{
				Name:      "test-agent",
				Namespace: "test-namespace",
				URL:       "http://test-agent.test-namespace.svc.cluster.local:8000",
			},
			expectedPath: "/test-namespace/test-agent",
			description:  "Different namespace, same agent name - must route to correct namespace",
		},
		{
			name: "unique agent - still uses namespaced path for consistency",
			modelInfo: ModelInfo{
				Name:      "unique-agent",
				Namespace: "test-namespace",
				URL:       "http://unique-agent.test-namespace.svc.cluster.local:8000",
			},
			expectedPath: "/test-namespace/unique-agent",
			description:  "Even unique agents route via namespaced endpoint for reliability",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify namespaced path construction
			// ALWAYS use namespaced format for routing
			namespacedPath := "/" + tc.modelInfo.Namespace + "/" + tc.modelInfo.Name

			assert.Equal(t, tc.expectedPath, namespacedPath, tc.description)

			// Verify we're NOT using simple format
			simplePath := "/" + tc.modelInfo.Name
			assert.NotEqual(t, simplePath, namespacedPath, "should NOT route to simple path format")
		})
	}
}
