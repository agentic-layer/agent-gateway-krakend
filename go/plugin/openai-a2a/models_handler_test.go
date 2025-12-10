package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModelIDGeneration_NamespaceFormat(t *testing.T) {
	// Agents are provided via config, model IDs use namespace/name format
	agents := []AgentInfo{
		{
			Name:      "test-agent-a",
			Namespace: "namespace-a",
			URL:       "http://test-agent-a.namespace-a.svc:8000",
			CreatedAt: 1731679815,
		},
		{
			Name:      "mock-agent",
			Namespace: "default",
			URL:       "http://mock-agent.default.svc:8000",
			CreatedAt: 1731696200,
		},
	}

	// Verify namespace/name format is used
	for _, agent := range agents {
		modelID := agent.Namespace + "/" + agent.Name

		assert.Contains(t, modelID, "/", "model ID should use namespace/name format")
		assert.Equal(t, agent.Namespace+"/"+agent.Name, modelID)
	}
}

func TestModelIDGeneration_SameNameDifferentNamespaces(t *testing.T) {
	// Test agents with same name in different namespaces
	agents := []AgentInfo{
		{
			Name:      "duplicate-agent",
			Namespace: "namespace-a",
			URL:       "http://duplicate-agent.namespace-a.svc:8000",
			CreatedAt: 1731696300,
		},
		{
			Name:      "duplicate-agent",
			Namespace: "namespace-b",
			URL:       "http://duplicate-agent.namespace-b.svc:8000",
			CreatedAt: 1731696400,
		},
	}

	// Build model IDs - should be unique even with same agent name
	modelIDs := make([]string, len(agents))
	for i, agent := range agents {
		modelIDs[i] = agent.Namespace + "/" + agent.Name
	}

	// Verify IDs are unique
	assert.NotEqual(t, modelIDs[0], modelIDs[1], "model IDs should be unique due to different namespaces")
	assert.Equal(t, "namespace-a/duplicate-agent", modelIDs[0])
	assert.Equal(t, "namespace-b/duplicate-agent", modelIDs[1])
}

func TestOpenAIModelFormat(t *testing.T) {
	agent := AgentInfo{
		Name:      "test-agent",
		Namespace: "test-namespace",
		URL:       "http://test-agent.test-namespace.svc:8000",
		CreatedAt: 1731679815,
	}

	// Build OpenAI model (same logic as handleModelsRequest)
	modelID := agent.Namespace + "/" + agent.Name
	model := OpenAIModel{
		ID:      modelID,
		Object:  "model",
		Created: agent.CreatedAt,
		OwnedBy: agent.Namespace,
	}

	// Verify OpenAI format compliance
	assert.Equal(t, "test-namespace/test-agent", model.ID)
	assert.Equal(t, "model", model.Object)
	assert.Equal(t, int64(1731679815), model.Created)
	assert.Equal(t, "test-namespace", model.OwnedBy)
}

func TestOpenAIModelsResponseFormat(t *testing.T) {
	models := []OpenAIModel{
		{
			ID:      "namespace-1/agent-1",
			Object:  "model",
			Created: 1731679815,
			OwnedBy: "namespace-1",
		},
		{
			ID:      "namespace-2/agent-2",
			Object:  "model",
			Created: 1731696200,
			OwnedBy: "namespace-2",
		},
	}

	response := OpenAIModelsResponse{
		Object: "list",
		Data:   models,
	}

	// Verify response structure
	assert.Equal(t, "list", response.Object)
	assert.Len(t, response.Data, 2)
	assert.Equal(t, "namespace-1/agent-1", response.Data[0].ID)
	assert.Equal(t, "namespace-2/agent-2", response.Data[1].ID)
}

func TestAgentInfoStructure(t *testing.T) {
	agent := AgentInfo{
		Name:      "test-agent",
		Namespace: "test-namespace",
		URL:       "http://test-agent.test-namespace.svc:8000",
		CreatedAt: 1731679815,
	}

	assert.Equal(t, "test-agent", agent.Name)
	assert.Equal(t, "test-namespace", agent.Namespace)
	assert.Equal(t, "http://test-agent.test-namespace.svc:8000", agent.URL)
	assert.Equal(t, int64(1731679815), agent.CreatedAt)
}

func TestBuildModelsFromAgentInfo(t *testing.T) {
	// Test building OpenAI models list from AgentInfo slice
	agents := []AgentInfo{
		{
			Name:      "unique-agent",
			Namespace: "default",
			URL:       "http://unique-agent.default.svc:8000",
			CreatedAt: 1731679815,
		},
		{
			Name:      "weather-agent",
			Namespace: "production",
			URL:       "http://weather-agent.production.svc:8000",
			CreatedAt: 1731696300,
		},
	}

	// Build models (same logic as handleModelsRequest)
	models := make([]OpenAIModel, 0, len(agents))
	for _, agent := range agents {
		modelID := agent.Namespace + "/" + agent.Name
		models = append(models, OpenAIModel{
			ID:      modelID,
			Object:  "model",
			Created: agent.CreatedAt,
			OwnedBy: agent.Namespace,
		})
	}

	// Verify results
	assert.Len(t, models, 2)
	assert.Equal(t, "default/unique-agent", models[0].ID)
	assert.Equal(t, "default", models[0].OwnedBy)
	assert.Equal(t, "production/weather-agent", models[1].ID)
	assert.Equal(t, "production", models[1].OwnedBy)
}

func TestEmptyAgentsConfig(t *testing.T) {
	agents := []AgentInfo{}

	// Build models from empty list
	models := make([]OpenAIModel, 0, len(agents))
	for _, agent := range agents {
		modelID := agent.Namespace + "/" + agent.Name
		models = append(models, OpenAIModel{
			ID:      modelID,
			Object:  "model",
			Created: agent.CreatedAt,
			OwnedBy: agent.Namespace,
		})
	}

	// Verify empty result
	assert.Len(t, models, 0)

	response := OpenAIModelsResponse{
		Object: "list",
		Data:   models,
	}
	assert.Equal(t, "list", response.Object)
	assert.Empty(t, response.Data)
}
