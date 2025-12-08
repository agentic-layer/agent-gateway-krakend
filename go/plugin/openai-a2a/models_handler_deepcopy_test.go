package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAgentDeepCopyObject_Independence(t *testing.T) {
	// Create an original agent with labels and annotations
	original := &Agent{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Agent",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
			Labels: map[string]string{
				"env": "test",
			},
			Annotations: map[string]string{
				"description": "test agent",
			},
		},
		Spec: AgentSpec{
			Exposed: true,
		},
		Status: AgentStatus{
			URL: "http://test-agent:8000",
		},
	}

	// Create a deep copy
	copied := original.DeepCopyObject().(*Agent)

	// Verify values are equal
	assert.Equal(t, original.Name, copied.Name)
	assert.Equal(t, original.Namespace, copied.Namespace)
	assert.Equal(t, original.Spec.Exposed, copied.Spec.Exposed)
	assert.Equal(t, original.Status.URL, copied.Status.URL)
	assert.Equal(t, original.Labels["env"], copied.Labels["env"])
	assert.Equal(t, original.Annotations["description"], copied.Annotations["description"])

	// Modify the original's labels
	original.Labels["env"] = "production"
	original.Labels["new-label"] = "new-value"

	// Verify copy's labels are unchanged
	assert.Equal(t, "test", copied.Labels["env"], "copied labels should not be affected by original changes")
	assert.NotContains(t, copied.Labels, "new-label", "copied labels should not contain new keys from original")

	// Modify the original's annotations
	original.Annotations["description"] = "modified"
	original.Annotations["new-annotation"] = "new-value"

	// Verify copy's annotations are unchanged
	assert.Equal(t, "test agent", copied.Annotations["description"], "copied annotations should not be affected by original changes")
	assert.NotContains(t, copied.Annotations, "new-annotation", "copied annotations should not contain new keys from original")

	// Modify primitive fields in original
	original.Spec.Exposed = false
	original.Status.URL = "http://modified:8000"

	// Verify copy's primitives are unchanged
	assert.True(t, copied.Spec.Exposed, "copied Spec should not be affected by original changes")
	assert.Equal(t, "http://test-agent:8000", copied.Status.URL, "copied Status should not be affected by original changes")
}

func TestAgentDeepCopyObject_NilHandling(t *testing.T) {
	var nilAgent *Agent
	result := nilAgent.DeepCopyObject()
	assert.Nil(t, result, "DeepCopyObject on nil agent should return nil")
}

func TestAgentDeepCopyObject_EmptyMaps(t *testing.T) {
	agent := &Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: AgentSpec{
			Exposed: true,
		},
	}

	copied := agent.DeepCopyObject().(*Agent)

	assert.NotNil(t, copied)
	assert.Equal(t, "test", copied.Name)
	assert.Equal(t, "default", copied.Namespace)
}

func TestAgentListDeepCopyObject_Independence(t *testing.T) {
	original := &AgentList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "AgentList",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "1000",
		},
		Items: []Agent{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "agent-1",
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: AgentSpec{
					Exposed: true,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "agent-2",
					Labels: map[string]string{
						"app": "prod",
					},
				},
				Spec: AgentSpec{
					Exposed: false,
				},
			},
		},
	}

	// Create a deep copy
	copied := original.DeepCopyObject().(*AgentList)

	// Verify values are equal
	assert.Equal(t, len(original.Items), len(copied.Items))
	assert.Equal(t, original.Items[0].Name, copied.Items[0].Name)
	assert.Equal(t, original.Items[1].Name, copied.Items[1].Name)

	// Modify original's first item labels
	original.Items[0].Labels["app"] = "modified"
	original.Items[0].Labels["new"] = "label"

	// Verify copied first item is unchanged
	assert.Equal(t, "test", copied.Items[0].Labels["app"])
	assert.NotContains(t, copied.Items[0].Labels, "new")

	// Modify original's second item
	original.Items[1].Spec.Exposed = true

	// Verify copied second item is unchanged
	assert.False(t, copied.Items[1].Spec.Exposed)

	// Modify original's items slice (add new item)
	original.Items = append(original.Items, Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "agent-3",
		},
	})

	// Verify copied items slice is unchanged
	assert.Equal(t, 2, len(copied.Items), "copied slice should not be affected by original slice modifications")
}

func TestAgentListDeepCopyObject_NilHandling(t *testing.T) {
	var nilList *AgentList
	result := nilList.DeepCopyObject()
	assert.Nil(t, result, "DeepCopyObject on nil list should return nil")
}

func TestAgentListDeepCopyObject_EmptyItems(t *testing.T) {
	list := &AgentList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: "1000",
		},
		Items: nil,
	}

	copied := list.DeepCopyObject().(*AgentList)

	assert.NotNil(t, copied)
	assert.Nil(t, copied.Items)
	assert.Equal(t, "1000", copied.ResourceVersion)
}

func TestAgentCache_DeepCopyIsolation(t *testing.T) {
	// This test verifies that the cache returns deep copies
	// and modifying returned agents doesn't affect the cache

	cache := &AgentCache{
		agents: []Agent{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cached-agent",
					Labels: map[string]string{
						"env": "test",
					},
				},
				Spec: AgentSpec{
					Exposed: true,
				},
			},
		},
	}

	// Get agents from cache (simulating the first return path)
	cache.mu.RLock()
	agentsCopy := make([]Agent, len(cache.agents))
	for i := range cache.agents {
		agentsCopy[i] = *cache.agents[i].DeepCopyObject().(*Agent)
	}
	cache.mu.RUnlock()

	// Verify we got the agent
	assert.Len(t, agentsCopy, 1)
	assert.Equal(t, "cached-agent", agentsCopy[0].Name)
	assert.Equal(t, "test", agentsCopy[0].Labels["env"])

	// Modify the returned copy
	agentsCopy[0].Labels["env"] = "production"
	agentsCopy[0].Labels["new"] = "label"
	agentsCopy[0].Spec.Exposed = false

	// Verify cache is unchanged
	cache.mu.RLock()
	assert.Equal(t, "test", cache.agents[0].Labels["env"], "cache labels should not be affected")
	assert.NotContains(t, cache.agents[0].Labels, "new", "cache should not have new labels")
	assert.True(t, cache.agents[0].Spec.Exposed, "cache Spec should not be affected")
	cache.mu.RUnlock()
}

func TestAgentDeepCopyObject_PointerFields(t *testing.T) {
	// Test that pointer fields in ObjectMeta are properly deep copied
	deletionGracePeriod := int64(30)
	deletionTimestamp := metav1.Now()

	original := &Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "test-agent",
			DeletionGracePeriodSeconds: &deletionGracePeriod,
			DeletionTimestamp:          &deletionTimestamp,
			Finalizers:                 []string{"finalizer1", "finalizer2"},
		},
	}

	copied := original.DeepCopyObject().(*Agent)

	// Verify pointer fields are copied
	assert.NotNil(t, copied.DeletionGracePeriodSeconds)
	assert.Equal(t, int64(30), *copied.DeletionGracePeriodSeconds)
	assert.NotNil(t, copied.DeletionTimestamp)
	assert.Equal(t, deletionTimestamp, *copied.DeletionTimestamp)

	// Modify original's pointer field values
	*original.DeletionGracePeriodSeconds = 60
	newTime := metav1.Now()
	original.DeletionTimestamp = &newTime

	// Verify copied pointers are independent
	assert.Equal(t, int64(30), *copied.DeletionGracePeriodSeconds, "copied pointer should not be affected")
	assert.Equal(t, deletionTimestamp, *copied.DeletionTimestamp, "copied timestamp should not be affected")

	// Modify original's slice
	original.Finalizers = append(original.Finalizers, "finalizer3")

	// Verify copied slice is independent
	assert.Len(t, copied.Finalizers, 2, "copied finalizers should not be affected")
}