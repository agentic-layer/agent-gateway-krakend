package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseModelParameter_ValidFormat(t *testing.T) {
	namespace, agentName, err := parseModelParameter("test-namespace/test-agent")

	assert.NoError(t, err)
	assert.Equal(t, "test-namespace", namespace)
	assert.Equal(t, "test-agent", agentName)
}

func TestParseModelParameter_SimpleFormat_Rejected(t *testing.T) {
	// Simple format (without namespace) is no longer supported
	_, _, err := parseModelParameter("test-agent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be in format 'namespace/agent-name'")
}

func TestParseModelParameter_EmptyModel(t *testing.T) {
	_, _, err := parseModelParameter("")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestParseModelParameter_MultipleSlashes(t *testing.T) {
	// Multiple slashes should be rejected
	_, _, err := parseModelParameter("namespace/agent/with/slashes")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid model format")
}

func TestParseModelParameter_PathTraversal(t *testing.T) {
	// Path traversal attempts should be rejected
	_, _, err := parseModelParameter("../namespace/agent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid model parameter format")
}

func TestParseModelParameter_EmptyNamespace(t *testing.T) {
	_, _, err := parseModelParameter("/test-agent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid model format")
}

func TestParseModelParameter_EmptyAgentName(t *testing.T) {
	_, _, err := parseModelParameter("test-namespace/")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid model format")
}

func TestParseModelParameter_InvalidNamespace(t *testing.T) {
	testCases := []struct {
		name        string
		model       string
		expectedErr string
	}{
		{
			name:        "uppercase namespace",
			model:       "TestNamespace/agent",
			expectedErr: "invalid namespace",
		},
		{
			name:        "namespace with underscore",
			model:       "test_namespace/agent",
			expectedErr: "invalid namespace",
		},
		{
			name:        "namespace starts with hyphen",
			model:       "-namespace/agent",
			expectedErr: "invalid namespace",
		},
		{
			name:        "namespace ends with hyphen",
			model:       "namespace-/agent",
			expectedErr: "invalid namespace",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseModelParameter(tc.model)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestParseModelParameter_InvalidAgentName(t *testing.T) {
	testCases := []struct {
		name        string
		model       string
		expectedErr string
	}{
		{
			name:        "uppercase agent name",
			model:       "namespace/TestAgent",
			expectedErr: "invalid agent name",
		},
		{
			name:        "agent name with underscore",
			model:       "namespace/test_agent",
			expectedErr: "invalid agent name",
		},
		{
			name:        "agent name starts with hyphen",
			model:       "namespace/-agent",
			expectedErr: "invalid agent name",
		},
		{
			name:        "agent name ends with hyphen",
			model:       "namespace/agent-",
			expectedErr: "invalid agent name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseModelParameter(tc.model)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestResolveAgentBackend_Found(t *testing.T) {
	agents := []AgentInfo{
		{
			Name:      "test-agent",
			Namespace: "namespace-a",
			URL:       "http://test-agent.namespace-a.svc.cluster.local:8000",
			CreatedAt: 1731679815,
		},
		{
			Name:      "test-agent",
			Namespace: "namespace-b",
			URL:       "http://test-agent.namespace-b.svc.cluster.local:8000",
			CreatedAt: 1731696200,
		},
	}

	// Resolve agent from namespace-a
	modelInfo, err := resolveAgentBackend("namespace-a/test-agent", agents)

	assert.NoError(t, err)
	assert.NotNil(t, modelInfo)
	assert.Equal(t, "test-agent", modelInfo.Name)
	assert.Equal(t, "namespace-a", modelInfo.Namespace)
	assert.Equal(t, "http://test-agent.namespace-a.svc.cluster.local:8000", modelInfo.URL)
}

func TestResolveAgentBackend_NotFound(t *testing.T) {
	agents := []AgentInfo{
		{
			Name:      "other-agent",
			Namespace: "default",
			URL:       "http://other-agent.default.svc:8000",
			CreatedAt: 1731679815,
		},
	}

	// Try to resolve non-existent agent
	_, err := resolveAgentBackend("default/non-existent", agents)

	assert.Error(t, err)
	resErr, ok := err.(*AgentResolutionError)
	assert.True(t, ok)
	assert.Equal(t, "not_found", resErr.Type)
	assert.Contains(t, resErr.ClientMsg, "model not found")
}

func TestResolveAgentBackend_WrongNamespace(t *testing.T) {
	agents := []AgentInfo{
		{
			Name:      "test-agent",
			Namespace: "namespace-a",
			URL:       "http://test-agent.namespace-a.svc:8000",
			CreatedAt: 1731679815,
		},
	}

	// Try to resolve with wrong namespace
	_, err := resolveAgentBackend("namespace-b/test-agent", agents)

	assert.Error(t, err)
	resErr, ok := err.(*AgentResolutionError)
	assert.True(t, ok)
	assert.Equal(t, "not_found", resErr.Type)
}

func TestResolveAgentBackend_MissingURL(t *testing.T) {
	agents := []AgentInfo{
		{
			Name:      "incomplete-agent",
			Namespace: "default",
			URL:       "", // Missing URL
			CreatedAt: 1731679815,
		},
	}

	_, err := resolveAgentBackend("default/incomplete-agent", agents)

	assert.Error(t, err)
	resErr, ok := err.(*AgentResolutionError)
	assert.True(t, ok)
	assert.Equal(t, "configuration_error", resErr.Type)
	assert.Contains(t, resErr.InternalMsg, "no URL configured")
}

func TestResolveAgentBackend_InvalidFormat(t *testing.T) {
	agents := []AgentInfo{
		{
			Name:      "test-agent",
			Namespace: "default",
			URL:       "http://test-agent.default.svc:8000",
			CreatedAt: 1731679815,
		},
	}

	// Simple name format should be rejected
	_, err := resolveAgentBackend("test-agent", agents)

	assert.Error(t, err)
	resErr, ok := err.(*AgentResolutionError)
	assert.True(t, ok)
	assert.Equal(t, "invalid_format", resErr.Type)
}

func TestResolveAgentBackend_EmptyAgentsList(t *testing.T) {
	agents := []AgentInfo{}

	_, err := resolveAgentBackend("default/test-agent", agents)

	assert.Error(t, err)
	resErr, ok := err.(*AgentResolutionError)
	assert.True(t, ok)
	assert.Equal(t, "not_found", resErr.Type)
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
}

func TestModelInfo_BackendURLExtraction(t *testing.T) {
	testCases := []struct {
		name        string
		agentURL    string
		expectedURL string
	}{
		{
			name:        "standard service URL",
			agentURL:    "http://agent.namespace.svc.cluster.local:8000",
			expectedURL: "http://agent.namespace.svc.cluster.local:8000",
		},
		{
			name:        "URL with different port",
			agentURL:    "http://agent.namespace.svc.cluster.local:9000",
			expectedURL: "http://agent.namespace.svc.cluster.local:9000",
		},
		{
			name:        "HTTPS URL",
			agentURL:    "https://agent.namespace.svc.cluster.local:8000",
			expectedURL: "https://agent.namespace.svc.cluster.local:8000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			agents := []AgentInfo{
				{
					Name:      "agent",
					Namespace: "namespace",
					URL:       tc.agentURL,
					CreatedAt: 1731679815,
				},
			}

			modelInfo, err := resolveAgentBackend("namespace/agent", agents)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedURL, modelInfo.URL)
		})
	}
}

func TestAgentPathConstruction(t *testing.T) {
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
			// This simulates the path construction in handleGlobalChatCompletions
			agentPath := "/" + tc.agentNamespace + "/" + tc.agentName

			assert.Equal(t, tc.expectedPath, agentPath)
			assert.Contains(t, agentPath, tc.agentNamespace, "path should contain namespace")
			assert.Contains(t, agentPath, tc.agentName, "path should contain agent name")
		})
	}
}

func TestRoutingPathFormat(t *testing.T) {
	testCases := []struct {
		name         string
		modelInfo    ModelInfo
		expectedPath string
	}{
		{
			name: "agent in default namespace",
			modelInfo: ModelInfo{
				Name:      "test-agent",
				Namespace: "default",
				URL:       "http://test-agent.default.svc.cluster.local:8000",
			},
			expectedPath: "/default/test-agent",
		},
		{
			name: "agent in custom namespace",
			modelInfo: ModelInfo{
				Name:      "test-agent",
				Namespace: "test-namespace",
				URL:       "http://test-agent.test-namespace.svc.cluster.local:8000",
			},
			expectedPath: "/test-namespace/test-agent",
		},
		{
			name: "agent with long name",
			modelInfo: ModelInfo{
				Name:      "weather-forecast-agent",
				Namespace: "production",
				URL:       "http://weather-forecast-agent.production.svc.cluster.local:8000",
			},
			expectedPath: "/production/weather-forecast-agent",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			agentPath := "/" + tc.modelInfo.Namespace + "/" + tc.modelInfo.Name

			assert.Equal(t, tc.expectedPath, agentPath)
		})
	}
}

func TestValidateK8sName(t *testing.T) {
	validNames := []string{
		"valid-name",
		"valid123",
		"a",
		"valid.name",
		"valid-name-123",
	}

	for _, name := range validNames {
		t.Run("valid_"+name, func(t *testing.T) {
			err := validateK8sName(name)
			assert.NoError(t, err)
		})
	}

	invalidNames := []struct {
		name        string
		expectedErr string
	}{
		{"", "cannot be empty"},
		{"-starts-with-hyphen", "must start with an alphanumeric"},
		{"ends-with-hyphen-", "must end with an alphanumeric"},
		{"Invalid-Uppercase", "must start with an alphanumeric"}, // Uppercase I is not alphanumeric (only a-z, 0-9)
		{"invalid_underscore", "invalid character"},
		{"invalid/slash", "invalid character"},
	}

	for _, tc := range invalidNames {
		t.Run("invalid_"+tc.name, func(t *testing.T) {
			err := validateK8sName(tc.name)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestAgentResolutionError_Structure(t *testing.T) {
	err := &AgentResolutionError{
		Type:        "not_found",
		InternalMsg: "agent default/test-agent not found",
		ClientMsg:   "model not found",
	}

	assert.Equal(t, "not_found", err.Type)
	assert.Equal(t, "agent default/test-agent not found", err.Error())
	assert.Equal(t, "model not found", err.ClientMsg)
}
