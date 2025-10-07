package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
	"github.com/stretchr/testify/assert"
)

const (
	configStr = `{
      "openai_a2a_config": {
        "endpoint": "/chat/completions"
      }
	}`
	configStrMinimal = `{
      "openai_a2a_config": {}
	}`
	configStrEmpty  = `{}`
	configStrCustom = `{
      "openai_a2a_config": {
        "endpoint": "/chat/completion"
      }
	}`
	configStrFaulty = `{
      "openai_a2a_config": {
        "endpoint": 123
      }
	}`
)

type MockHandler struct {
	ReceivedRequest *http.Request
	ReceivedBody    []byte
	Response        []byte
	StatusCode      int
}

func (h *MockHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	h.ReceivedRequest = req
	if req.Body != nil {
		h.ReceivedBody, _ = io.ReadAll(req.Body)
	}

	if h.StatusCode == 0 {
		h.StatusCode = http.StatusOK
	}

	writer.WriteHeader(h.StatusCode)
	if h.Response != nil {
		writer.Write(h.Response)
	}
}

func TestOpenAIToA2ATransformation(t *testing.T) {
	var extraConfig map[string]interface{}
	json.Unmarshal([]byte(configStr), &extraConfig)

	mockA2AResponse := models.A2AResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: models.A2AResult{
			Artifacts: []models.A2AArtifact{
				{
					ArtifactID: "artifact-123",
					Parts: []models.A2APart{
						{Kind: "text", Text: "The weather in New York is sunny."},
					},
				},
			},
			ContextID: "context-123",
			History: []models.A2AHistoryMessage{
				{
					Kind:      "message",
					MessageID: "msg-1",
					Role:      "user",
					Parts: []models.A2APart{
						{Kind: "text", Text: "What is the weather in New York?"},
					},
				},
				{
					Kind:      "message",
					MessageID: "msg-2",
					Role:      "agent",
					Parts: []models.A2APart{
						{Kind: "text", Text: "The weather in New York is sunny."},
					},
				},
			},
			ID:   "task-123",
			Kind: "task",
			Status: models.A2AStatus{
				State:     "completed",
				Timestamp: "2025-10-02T12:00:00Z",
			},
		},
	}
	mockResponseBytes, _ := json.Marshal(mockA2AResponse)

	mockHandler := &MockHandler{
		Response: mockResponseBytes,
	}

	handlers, _ := HandlerRegisterer.registerHandlers(context.Background(), extraConfig, mockHandler)
	ts := httptest.NewUnstartedServer(handlers)
	ts.Start()
	defer ts.Close()

	openAIRequest := models.OpenAIRequest{
		Model: "gpt-4",
		Messages: []models.OpenAIMessage{
			{Role: "user", Content: "What is the weather in New York?"},
		},
	}
	reqBody, _ := json.Marshal(openAIRequest)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/weather-agent/chat/completions", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	resp, err := client.Do(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify the transformed request was sent to backend
	assert.NotNil(t, mockHandler.ReceivedRequest)
	assert.Equal(t, "/weather-agent", mockHandler.ReceivedRequest.URL.Path)

	// Verify A2A format was sent to backend
	var a2aReq models.A2ARequest
	err = json.Unmarshal(mockHandler.ReceivedBody, &a2aReq)
	assert.NoError(t, err)
	assert.Equal(t, "2.0", a2aReq.JSONRPC)
	assert.Equal(t, "message/send", a2aReq.Method)
	assert.Equal(t, "user", a2aReq.Params.Message.Role)
	assert.Equal(t, 1, len(a2aReq.Params.Message.Parts))
	assert.Equal(t, "text", a2aReq.Params.Message.Parts[0].Kind)
	assert.Equal(t, "What is the weather in New York?", a2aReq.Params.Message.Parts[0].Text)
	assert.NotEmpty(t, a2aReq.Params.Message.MessageID)
	assert.NotEmpty(t, a2aReq.Params.Message.ContextID)

	// Verify OpenAI response format was returned to client
	var openAIResp models.OpenAIResponse
	respBody, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(respBody, &openAIResp)
	assert.NoError(t, err)
	assert.Equal(t, "chat.completion", openAIResp.Object)
	assert.Equal(t, "gpt-4", openAIResp.Model)
	assert.Equal(t, 1, len(openAIResp.Choices))
	assert.Equal(t, "assistant", openAIResp.Choices[0].Message.Role)
	assert.Equal(t, "The weather in New York is sunny.", openAIResp.Choices[0].Message.Content)
	assert.Equal(t, "stop", openAIResp.Choices[0].FinishReason)
}

func TestNonChatCompletionsEndpointPassthrough(t *testing.T) {
	var extraConfig map[string]interface{}
	json.Unmarshal([]byte(configStr), &extraConfig)

	mockHandler := &MockHandler{
		StatusCode: http.StatusNotFound,
	}

	handlers, _ := HandlerRegisterer.registerHandlers(context.Background(), extraConfig, mockHandler)
	ts := httptest.NewUnstartedServer(handlers)
	ts.Start()
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/other-endpoint", nil)
	client := &http.Client{}

	resp, err := client.Do(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "/other-endpoint", mockHandler.ReceivedRequest.URL.Path)
}

func TestCustomEndpointConfiguration(t *testing.T) {
	var extraConfig map[string]interface{}
	json.Unmarshal([]byte(configStrCustom), &extraConfig)

	mockA2AResponse := models.A2AResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: models.A2AResult{
			Artifacts: []models.A2AArtifact{
				{
					ArtifactID: "artifact-456",
					Parts: []models.A2APart{
						{Kind: "text", Text: "Response"},
					},
				},
			},
			ContextID: "context-456",
			History: []models.A2AHistoryMessage{
				{
					Kind:      "message",
					MessageID: "msg-1",
					Role:      "user",
					Parts: []models.A2APart{
						{Kind: "text", Text: "Test"},
					},
				},
				{
					Kind:      "message",
					MessageID: "msg-2",
					Role:      "agent",
					Parts: []models.A2APart{
						{Kind: "text", Text: "Response"},
					},
				},
			},
			ID:   "task-456",
			Kind: "task",
			Status: models.A2AStatus{
				State:     "completed",
				Timestamp: "2025-10-02T12:00:00Z",
			},
		},
	}
	mockResponseBytes, _ := json.Marshal(mockA2AResponse)

	mockHandler := &MockHandler{
		Response: mockResponseBytes,
	}

	handlers, _ := HandlerRegisterer.registerHandlers(context.Background(), extraConfig, mockHandler)
	ts := httptest.NewUnstartedServer(handlers)
	ts.Start()
	defer ts.Close()

	openAIRequest := models.OpenAIRequest{
		Model:    "gpt-4",
		Messages: []models.OpenAIMessage{{Role: "user", Content: "Test"}},
	}
	reqBody, _ := json.Marshal(openAIRequest)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/agent/chat/completion", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	resp, err := client.Do(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/agent", mockHandler.ReceivedRequest.URL.Path)
}

func TestInvalidOpenAIRequest(t *testing.T) {
	var extraConfig map[string]interface{}
	json.Unmarshal([]byte(configStr), &extraConfig)

	mockHandler := &MockHandler{}

	handlers, _ := HandlerRegisterer.registerHandlers(context.Background(), extraConfig, mockHandler)
	ts := httptest.NewUnstartedServer(handlers)
	ts.Start()
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/agent/chat/completions", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	resp, err := client.Do(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func Test_parseConfig_returns_config_when_valid(t *testing.T) {
	// given
	var extraConfig map[string]interface{}
	json.Unmarshal([]byte(configStr), &extraConfig)
	var cfg config

	// when
	err := parseConfig(extraConfig, &cfg)

	// then
	assert.NoError(t, err)
	assert.Equal(t, "/chat/completions", cfg.Endpoint)
}

func Test_parseConfig_minimal_returns_config_when_valid(t *testing.T) {
	// given
	var extraConfig map[string]interface{}
	json.Unmarshal([]byte(configStrMinimal), &extraConfig)
	var cfg config

	// when
	err := parseConfig(extraConfig, &cfg)

	// then
	assert.NoError(t, err)
	assert.Equal(t, "/chat/completions", cfg.Endpoint)
}

func Test_parseConfig_returns_config_when_empty(t *testing.T) {
	// given
	var extraConfig map[string]interface{}
	json.Unmarshal([]byte(configStrEmpty), &extraConfig)
	var cfg config

	// when
	err := parseConfig(extraConfig, &cfg)

	// then
	assert.NoError(t, err)
	assert.Equal(t, "/chat/completions", cfg.Endpoint)
}

func Test_parseConfig_returns_error_when_invalid(t *testing.T) {
	// given
	var extraConfig map[string]interface{}
	json.Unmarshal([]byte(configStrFaulty), &extraConfig)
	var cfg config

	// when
	err := parseConfig(extraConfig, &cfg)

	// then
	assert.Error(t, err)
}

func Test_isChatCompletionsEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		endpoint string
		expected bool
	}{
		{"standard endpoint", "/agent/chat/completions", "/chat/completions", true},
		{"custom endpoint", "/agent/chat/completion", "/chat/completion", true},
		{"non-matching path", "/agent/other", "/chat/completions", false},
		{"root path", "/chat/completions", "/chat/completions", false},
		{"nested path", "/agent/foo/chat/completions", "/chat/completions", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isChatCompletionsEndpoint(tt.path, tt.endpoint)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_extractPathPrefix(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		endpoint string
		expected string
	}{
		{"standard path", "/weather-agent/chat/completions", "/chat/completions", "weather-agent"},
		{"custom endpoint", "/my-agent/chat/completion", "/chat/completion", "my-agent"},
		{"no match", "/agent/other", "/chat/completions", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPathPrefix(tt.path, tt.endpoint)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_transformOpenAIToA2A(t *testing.T) {
	openAIReq := models.OpenAIRequest{
		Model: "gpt-4",
		Messages: []models.OpenAIMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "What is the weather?"},
		},
		Temperature: 0.7,
	}

	a2aReq := transformOpenAIToA2A(openAIReq)

	assert.Equal(t, "2.0", a2aReq.JSONRPC)
	assert.Equal(t, 1, a2aReq.ID)
	assert.Equal(t, "message/send", a2aReq.Method)
	assert.Equal(t, "user", a2aReq.Params.Message.Role)
	assert.Equal(t, 1, len(a2aReq.Params.Message.Parts))
	assert.Equal(t, "text", a2aReq.Params.Message.Parts[0].Kind)
	assert.Equal(t, "What is the weather?", a2aReq.Params.Message.Parts[0].Text)
	assert.NotEmpty(t, a2aReq.Params.Message.MessageID)
	assert.NotEmpty(t, a2aReq.Params.Message.ContextID)
	assert.NotNil(t, a2aReq.Params.Metadata)

	// Verify history contains the system message
	assert.Equal(t, 1, len(a2aReq.Params.History))
	assert.Equal(t, "system", a2aReq.Params.History[0].Role)
	assert.Equal(t, "You are a helpful assistant.", a2aReq.Params.History[0].Parts[0].Text)
}

func Test_transformOpenAIToA2A_WithMultipleMessages(t *testing.T) {
	openAIReq := models.OpenAIRequest{
		Model: "gpt-4",
		Messages: []models.OpenAIMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "What is the weather?"},
			{Role: "assistant", Content: "I'll check the weather for you."},
			{Role: "user", Content: "What about tomorrow?"},
		},
	}

	a2aReq := transformOpenAIToA2A(openAIReq)

	// Last message should be the primary message
	assert.Equal(t, "user", a2aReq.Params.Message.Role)
	assert.Equal(t, "What about tomorrow?", a2aReq.Params.Message.Parts[0].Text)

	// History should contain all previous messages
	assert.Equal(t, 3, len(a2aReq.Params.History))

	// Verify first message (system)
	assert.Equal(t, "system", a2aReq.Params.History[0].Role)
	assert.Equal(t, "You are a helpful assistant.", a2aReq.Params.History[0].Parts[0].Text)

	// Verify second message (user)
	assert.Equal(t, "user", a2aReq.Params.History[1].Role)
	assert.Equal(t, "What is the weather?", a2aReq.Params.History[1].Parts[0].Text)

	// Verify third message (assistant -> agent)
	assert.Equal(t, "agent", a2aReq.Params.History[2].Role)
	assert.Equal(t, "I'll check the weather for you.", a2aReq.Params.History[2].Parts[0].Text)

	// All messages should share the same contextId
	contextID := a2aReq.Params.Message.ContextID
	for _, histMsg := range a2aReq.Params.History {
		assert.Equal(t, contextID, histMsg.ContextID)
	}
}

func Test_transformA2AToOpenAI_WithArtifacts(t *testing.T) {
	a2aResp := models.A2AResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: models.A2AResult{
			Artifacts: []models.A2AArtifact{
				{
					ArtifactID: "artifact-123",
					Parts: []models.A2APart{
						{Kind: "text", Text: "The weather is sunny."},
					},
				},
			},
			ContextID: "context-123",
			History: []models.A2AHistoryMessage{
				{
					Kind:      "message",
					MessageID: "msg-1",
					Role:      "user",
					Parts: []models.A2APart{
						{Kind: "text", Text: "What is the weather?"},
					},
				},
				{
					Kind:      "message",
					MessageID: "msg-2",
					Role:      "agent",
					Parts: []models.A2APart{
						{Kind: "text", Text: "The weather is sunny."},
					},
				},
			},
			ID:   "task-123",
			Kind: "task",
			Status: models.A2AStatus{
				State:     "completed",
				Timestamp: "2025-10-02T12:00:00Z",
			},
		},
	}

	openAIReq := models.OpenAIRequest{
		Model: "gpt-4",
	}

	openAIResp := transformA2AToOpenAI(a2aResp, openAIReq)

	assert.Equal(t, "chat.completion", openAIResp.Object)
	assert.Equal(t, "gpt-4", openAIResp.Model)
	assert.Equal(t, 1, len(openAIResp.Choices))
	assert.Equal(t, 0, openAIResp.Choices[0].Index)
	assert.Equal(t, "assistant", openAIResp.Choices[0].Message.Role)
	assert.Equal(t, "The weather is sunny.", openAIResp.Choices[0].Message.Content)
	assert.Equal(t, "stop", openAIResp.Choices[0].FinishReason)
	assert.NotEmpty(t, openAIResp.ID)
	assert.NotZero(t, openAIResp.Created)
}

func Test_transformA2AToOpenAI_WithMultipleArtifacts(t *testing.T) {
	a2aResp := models.A2AResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: models.A2AResult{
			Artifacts: []models.A2AArtifact{
				{
					ArtifactID: "artifact-1",
					Parts: []models.A2APart{
						{Kind: "text", Text: "First part. "},
					},
				},
				{
					ArtifactID: "artifact-2",
					Parts: []models.A2APart{
						{Kind: "text", Text: "Second part."},
					},
				},
			},
			ContextID: "context-123",
			History:   []models.A2AHistoryMessage{},
			ID:        "task-123",
			Kind:      "task",
			Status: models.A2AStatus{
				State:     "completed",
				Timestamp: "2025-10-02T12:00:00Z",
			},
		},
	}

	openAIReq := models.OpenAIRequest{Model: "gpt-4"}
	openAIResp := transformA2AToOpenAI(a2aResp, openAIReq)

	assert.Equal(t, "First part. Second part.", openAIResp.Choices[0].Message.Content)
}

func Test_transformA2AToOpenAI_FallbackToHistory(t *testing.T) {
	a2aResp := models.A2AResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: models.A2AResult{
			Artifacts: []models.A2AArtifact{}, // No artifacts
			ContextID: "context-123",
			History: []models.A2AHistoryMessage{
				{
					Kind:      "message",
					MessageID: "msg-1",
					Role:      "user",
					Parts: []models.A2APart{
						{Kind: "text", Text: "What is the weather?"},
					},
				},
				{
					Kind:      "message",
					MessageID: "msg-2",
					Role:      "agent",
					Parts: []models.A2APart{
						{Kind: "text", Text: "The weather is sunny."},
					},
				},
			},
			ID:   "task-123",
			Kind: "task",
			Status: models.A2AStatus{
				State:     "completed",
				Timestamp: "2025-10-02T12:00:00Z",
			},
		},
	}

	openAIReq := models.OpenAIRequest{Model: "gpt-4"}
	openAIResp := transformA2AToOpenAI(a2aResp, openAIReq)

	assert.Equal(t, "assistant", openAIResp.Choices[0].Message.Role)
	assert.Equal(t, "The weather is sunny.", openAIResp.Choices[0].Message.Content)
}

func Test_transformA2AToOpenAI_SkipsNonTextParts(t *testing.T) {
	a2aResp := models.A2AResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: models.A2AResult{
			Artifacts: []models.A2AArtifact{
				{
					ArtifactID: "artifact-123",
					Parts: []models.A2APart{
						{Kind: "data", Data: map[string]string{"foo": "bar"}},
						{Kind: "text", Text: "Visible text"},
						{Kind: "image", Data: "base64data"},
					},
				},
			},
			ContextID: "context-123",
			History:   []models.A2AHistoryMessage{},
			ID:        "task-123",
			Kind:      "task",
			Status: models.A2AStatus{
				State:     "completed",
				Timestamp: "2025-10-02T12:00:00Z",
			},
		},
	}

	openAIReq := models.OpenAIRequest{Model: "gpt-4"}
	openAIResp := transformA2AToOpenAI(a2aResp, openAIReq)

	assert.Equal(t, "Visible text", openAIResp.Choices[0].Message.Content)
}
