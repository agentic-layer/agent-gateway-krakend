package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestModelIDGeneration_UniqueNames(t *testing.T) {
	// Simulate agents with unique names
	agents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-agent-a",
				Namespace:         "test-namespace-a",
				CreationTimestamp: metav1.Time{Time: time.Unix(1731679815, 0)},
			},
			Spec: AgentSpec{Exposed: true},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "mock-agent",
				Namespace:         "default",
				CreationTimestamp: metav1.Time{Time: time.Unix(1731696200, 0)},
			},
			Spec: AgentSpec{Exposed: true},
		},
	}

	// Detect name collisions (same logic as handleModelsRequest)
	nameCount := make(map[string]int)
	for _, agent := range agents {
		nameCount[agent.Name]++
	}

	// Verify simple format is used for unique names
	for _, agent := range agents {
		var modelID string
		if nameCount[agent.Name] > 1 {
			modelID = agent.Namespace + "/" + agent.Name
		} else {
			modelID = agent.Name
		}

		assert.NotContains(t, modelID, "/", "unique names should use simple format without namespace")
		assert.Equal(t, agent.Name, modelID)
	}
}

func TestModelIDGeneration_NameCollisions(t *testing.T) {
	// Simulate agents with duplicate names in different namespaces
	agents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "duplicate-agent",
				Namespace:         "namespace-a",
				CreationTimestamp: metav1.Time{Time: time.Unix(1731696300, 0)},
			},
			Spec: AgentSpec{Exposed: true},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "duplicate-agent",
				Namespace:         "namespace-b",
				CreationTimestamp: metav1.Time{Time: time.Unix(1731696400, 0)},
			},
			Spec: AgentSpec{Exposed: true},
		},
	}

	// Detect name collisions
	nameCount := make(map[string]int)
	for _, agent := range agents {
		nameCount[agent.Name]++
	}

	// Verify namespaced format is used for collisions
	expectedIDs := []string{"namespace-a/duplicate-agent", "namespace-b/duplicate-agent"}
	for i, agent := range agents {
		var modelID string
		if nameCount[agent.Name] > 1 {
			modelID = agent.Namespace + "/" + agent.Name
		} else {
			modelID = agent.Name
		}

		assert.Contains(t, modelID, "/", "colliding names should use namespaced format")
		assert.Equal(t, expectedIDs[i], modelID)
	}
}

func TestModelIDGeneration_MixedScenario(t *testing.T) {
	// Simulate mixed scenario: some unique, some duplicate names
	agents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "unique-agent",
				Namespace:         "default",
				CreationTimestamp: metav1.Time{Time: time.Unix(1731679815, 0)},
			},
			Spec: AgentSpec{Exposed: true},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "duplicate-agent",
				Namespace:         "namespace-a",
				CreationTimestamp: metav1.Time{Time: time.Unix(1731696300, 0)},
			},
			Spec: AgentSpec{Exposed: true},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "duplicate-agent",
				Namespace:         "namespace-b",
				CreationTimestamp: metav1.Time{Time: time.Unix(1731696400, 0)},
			},
			Spec: AgentSpec{Exposed: true},
		},
	}

	// Detect name collisions
	nameCount := make(map[string]int)
	for _, agent := range agents {
		nameCount[agent.Name]++
	}

	// Build model IDs
	var modelIDs []string
	for _, agent := range agents {
		var modelID string
		if nameCount[agent.Name] > 1 {
			modelID = agent.Namespace + "/" + agent.Name
		} else {
			modelID = agent.Name
		}
		modelIDs = append(modelIDs, modelID)
	}

	// Verify results
	assert.Equal(t, "unique-agent", modelIDs[0], "unique name should use simple format")
	assert.Equal(t, "namespace-a/duplicate-agent", modelIDs[1], "duplicate should use namespaced format")
	assert.Equal(t, "namespace-b/duplicate-agent", modelIDs[2], "duplicate should use namespaced format")
}

func TestOpenAIModelFormat(t *testing.T) {
	agent := Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-agent",
			Namespace:         "test-namespace",
			CreationTimestamp: metav1.Time{Time: time.Unix(1731679815, 0)},
		},
		Spec: AgentSpec{Exposed: true},
	}

	model := OpenAIModel{
		ID:      agent.Name,
		Object:  "model",
		Created: agent.CreationTimestamp.Unix(),
		OwnedBy: agent.Namespace,
	}

	// Verify OpenAI format compliance
	assert.Equal(t, "test-agent", model.ID)
	assert.Equal(t, "model", model.Object)
	assert.Equal(t, int64(1731679815), model.Created)
	assert.Equal(t, "test-namespace", model.OwnedBy)
}

func TestOpenAIModelsResponseFormat(t *testing.T) {
	models := []OpenAIModel{
		{
			ID:      "agent-1",
			Object:  "model",
			Created: 1731679815,
			OwnedBy: "namespace-1",
		},
		{
			ID:      "agent-2",
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
	assert.Equal(t, "agent-1", response.Data[0].ID)
	assert.Equal(t, "agent-2", response.Data[1].ID)
}

func TestExposedAgentFiltering(t *testing.T) {
	// Simulate a mix of exposed and non-exposed agents
	allAgents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "exposed-1", Namespace: "default"},
			Spec:       AgentSpec{Exposed: true},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "not-exposed", Namespace: "default"},
			Spec:       AgentSpec{Exposed: false},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "exposed-2", Namespace: "default"},
			Spec:       AgentSpec{Exposed: true},
		},
	}

	// Filter for exposed agents (same logic as K8sClient.ListExposedAgents)
	exposedAgents := make([]Agent, 0)
	for _, agent := range allAgents {
		if agent.Spec.Exposed {
			exposedAgents = append(exposedAgents, agent)
		}
	}

	// Verify filtering
	assert.Len(t, exposedAgents, 2)
	assert.Equal(t, "exposed-1", exposedAgents[0].Name)
	assert.Equal(t, "exposed-2", exposedAgents[1].Name)
}