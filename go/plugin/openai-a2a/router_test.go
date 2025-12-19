package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveAgentBackend_Found(t *testing.T) {
	agents := []AgentInfo{
		{
			ModelID:   "test-agent-v1",
			URL:       "http://localhost:8001",
			OwnedBy:   "test-corp",
			CreatedAt: 1731679815,
		},
		{
			ModelID:   "prod/weather-agent",
			URL:       "http://localhost:8002",
			OwnedBy:   "weather-team",
			CreatedAt: 1731696200,
		},
	}

	modelInfo, err := resolveAgentBackend("test-agent-v1", agents)

	assert.NoError(t, err)
	assert.NotNil(t, modelInfo)
	assert.Equal(t, "test-agent-v1", modelInfo.ModelID)
	assert.Equal(t, "/test-agent-v1", modelInfo.Path)
	assert.Equal(t, "http://localhost:8001", modelInfo.URL)
}

func TestResolveAgentBackend_NotFound(t *testing.T) {
	agents := []AgentInfo{
		{
			ModelID:   "agent-alpha",
			URL:       "http://localhost:8001",
			OwnedBy:   "team-a",
			CreatedAt: 1731679815,
		},
	}

	_, err := resolveAgentBackend("non-existent-agent", agents)

	assert.Error(t, err)
	resErr, ok := err.(*AgentResolutionError)
	assert.True(t, ok)
	assert.Equal(t, "not_found", resErr.Type)
	assert.Contains(t, resErr.ClientMsg, "model not found")
}

func TestResolveAgentBackend_EmptyModel(t *testing.T) {
	agents := []AgentInfo{
		{
			ModelID:   "my-agent",
			URL:       "http://localhost:8001",
			OwnedBy:   "engineering",
			CreatedAt: 1731679815,
		},
	}

	_, err := resolveAgentBackend("", agents)

	assert.Error(t, err)
	resErr, ok := err.(*AgentResolutionError)
	assert.True(t, ok)
	assert.Equal(t, "invalid_format", resErr.Type)
}

func TestResolveAgentBackend_PathTraversal(t *testing.T) {
	agents := []AgentInfo{
		{
			ModelID:   "secure-agent",
			URL:       "http://localhost:8001",
			OwnedBy:   "security",
			CreatedAt: 1731679815,
		},
	}

	_, err := resolveAgentBackend("../etc/passwd", agents)

	assert.Error(t, err)
	resErr, ok := err.(*AgentResolutionError)
	assert.True(t, ok)
	assert.Equal(t, "invalid_format", resErr.Type)
	assert.Contains(t, resErr.InternalMsg, "invalid pattern '..'")
}

func TestResolveAgentBackend_MissingURL(t *testing.T) {
	agents := []AgentInfo{
		{
			ModelID:   "org/incomplete-agent",
			URL:       "", // Missing URL
			OwnedBy:   "org",
			CreatedAt: 1731679815,
		},
	}

	_, err := resolveAgentBackend("org/incomplete-agent", agents)

	assert.Error(t, err)
	resErr, ok := err.(*AgentResolutionError)
	assert.True(t, ok)
	assert.Equal(t, "configuration_error", resErr.Type)
	assert.Contains(t, resErr.InternalMsg, "no URL configured")
}

func TestResolveAgentBackend_EmptyAgentsList(t *testing.T) {
	agents := []AgentInfo{}

	_, err := resolveAgentBackend("any-agent-id", agents)

	assert.Error(t, err)
	resErr, ok := err.(*AgentResolutionError)
	assert.True(t, ok)
	assert.Equal(t, "not_found", resErr.Type)
}