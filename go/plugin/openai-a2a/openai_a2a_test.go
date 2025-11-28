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

	timestamp := "2025-10-02T12:00:00Z"
	mockA2AResponse := models.SendMessageSuccessResponse{
		Jsonrpc: "2.0",
		Id:      1,
		Result: models.SendMessageSuccessResponseResult{
			Artifacts: []models.Artifact{
				{
					ArtifactId: "artifact-123",
					Parts: []models.ArtifactPartsElem{
						models.TextPart{Kind: "text", Text: "The weather in New York is sunny."},
					},
				},
			},
			ContextId: "context-123",
			History: []models.Message{
				{
					Kind:      "message",
					MessageId: "msg-1",
					Role:      "user",
					Parts: []models.MessagePartsElem{
						models.TextPart{Kind: "text", Text: "What is the weather in New York?"},
					},
				},
				{
					Kind:      "message",
					MessageId: "msg-2",
					Role:      "agent",
					Parts: []models.MessagePartsElem{
						models.TextPart{Kind: "text", Text: "The weather in New York is sunny."},
					},
				},
			},
			Id:        "task-123",
			Kind:      "task",
			MessageId: "msg-2",
			Role:      "agent",
			Parts: []models.MessagePartsElem{
				models.TextPart{Kind: "text", Text: "The weather in New York is sunny."},
			},
			Status: models.TaskStatus{
				State:     "completed",
				Timestamp: &timestamp,
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
	var a2aReq models.SendMessageRequest
	err = json.Unmarshal(mockHandler.ReceivedBody, &a2aReq)
	assert.NoError(t, err)
	assert.Equal(t, "2.0", a2aReq.Jsonrpc)
	assert.Equal(t, "message/send", a2aReq.Method)
	assert.Equal(t, "message", a2aReq.Params.Message.Kind)
	assert.Equal(t, 1, len(a2aReq.Params.Message.Parts))

	// Cast the part to TextPart to access fields
	textPart, ok := a2aReq.Params.Message.Parts[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "text", textPart["kind"])
	assert.Equal(t, "What is the weather in New York?", textPart["text"])
	assert.NotEmpty(t, a2aReq.Params.Message.MessageId)
	assert.NotNil(t, a2aReq.Params.Message.ContextId)

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

	timestamp := "2025-10-02T12:00:00Z"
	mockA2AResponse := models.SendMessageSuccessResponse{
		Jsonrpc: "2.0",
		Id:      1,
		Result: models.SendMessageSuccessResponseResult{
			Artifacts: []models.Artifact{
				{
					ArtifactId: "artifact-456",
					Parts: []models.ArtifactPartsElem{
						models.TextPart{Kind: "text", Text: "Response"},
					},
				},
			},
			ContextId: "context-456",
			History: []models.Message{
				{
					Kind:      "message",
					MessageId: "msg-1",
					Role:      "user",
					Parts: []models.MessagePartsElem{
						models.TextPart{Kind: "text", Text: "Test"},
					},
				},
				{
					Kind:      "message",
					MessageId: "msg-2",
					Role:      "agent",
					Parts: []models.MessagePartsElem{
						models.TextPart{Kind: "text", Text: "Response"},
					},
				},
			},
			Id:        "task-456",
			Kind:      "task",
			MessageId: "msg-2",
			Role:      "agent",
			Parts: []models.MessagePartsElem{
				models.TextPart{Kind: "text", Text: "Response"},
			},
			Status: models.TaskStatus{
				State:     "completed",
				Timestamp: &timestamp,
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
			{Role: "user", Content: "What is the weather?"},
		},
		Temperature: 0.7,
	}

	a2aReq, err := transformOpenAIToA2A(openAIReq, "conversionId")

	assert.Nil(t, err)
	assert.Equal(t, "2.0", a2aReq.Jsonrpc)
	assert.Equal(t, 1, a2aReq.Id)
	assert.Equal(t, "message/send", a2aReq.Method)
	assert.Equal(t, "message", a2aReq.Params.Message.Kind)
	assert.Equal(t, 1, len(a2aReq.Params.Message.Parts))

	textPart := a2aReq.Params.Message.Parts[0].(models.TextPart)
	assert.Equal(t, "text", textPart.Kind)
	assert.Equal(t, "What is the weather?", textPart.Text)
	assert.NotEmpty(t, a2aReq.Params.Message.MessageId)
	assert.NotNil(t, a2aReq.Params.Message.ContextId)
	assert.NotNil(t, a2aReq.Params.Metadata)
}

func Test_transformOpenAIToA2A_WithMultipleMessages(t *testing.T) {
	openAIReq := models.OpenAIRequest{
		Model: "gpt-4",
		Messages: []models.OpenAIMessage{
			{Role: "user", Content: "What about tomorrow?"},
		},
	}

	a2aReq, err := transformOpenAIToA2A(openAIReq, "conversionId")

	assert.Nil(t, err)

	// Last message should be the primary message
	assert.Equal(t, "message", a2aReq.Params.Message.Kind)
	textPart := a2aReq.Params.Message.Parts[0].(models.TextPart)
	assert.Equal(t, "What about tomorrow?", textPart.Text)

	assert.NotNil(t, a2aReq.Params.Message.ContextId)
}

func Test_transformA2AToOpenAI_WithArtifacts(t *testing.T) {
	timestamp := "2025-10-02T12:00:00Z"
	a2aResp := models.SendMessageSuccessResponse{
		Jsonrpc: "2.0",
		Id:      1,
		Result: models.SendMessageSuccessResponseResult{
			Artifacts: []models.Artifact{
				{
					ArtifactId: "artifact-123",
					Parts: []models.ArtifactPartsElem{
						models.TextPart{Kind: "text", Text: "The weather is sunny."},
					},
				},
			},
			ContextId: "context-123",
			History: []models.Message{
				{
					Kind:      "message",
					MessageId: "msg-1",
					Role:      "user",
					Parts: []models.MessagePartsElem{
						models.TextPart{Kind: "text", Text: "What is the weather?"},
					},
				},
				{
					Kind:      "message",
					MessageId: "msg-2",
					Role:      "agent",
					Parts: []models.MessagePartsElem{
						models.TextPart{Kind: "text", Text: "The weather is sunny."},
					},
				},
			},
			Id:        "task-123",
			Kind:      "task",
			MessageId: "msg-2",
			Role:      "agent",
			Parts: []models.MessagePartsElem{
				models.TextPart{Kind: "text", Text: "The weather is sunny."},
			},
			Status: models.TaskStatus{
				State:     "completed",
				Timestamp: &timestamp,
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
	timestamp := "2025-10-02T12:00:00Z"
	a2aResp := models.SendMessageSuccessResponse{
		Jsonrpc: "2.0",
		Id:      1,
		Result: models.SendMessageSuccessResponseResult{
			Artifacts: []models.Artifact{
				{
					ArtifactId: "artifact-1",
					Parts: []models.ArtifactPartsElem{
						models.TextPart{Kind: "text", Text: "First part. "},
					},
				},
				{
					ArtifactId: "artifact-2",
					Parts: []models.ArtifactPartsElem{
						models.TextPart{Kind: "text", Text: "Second part."},
					},
				},
			},
			ContextId: "context-123",
			History:   []models.Message{},
			Id:        "task-123",
			Kind:      "task",
			MessageId: "msg-1",
			Role:      "agent",
			Parts: []models.MessagePartsElem{
				models.TextPart{Kind: "text", Text: "First part. Second part."},
			},
			Status: models.TaskStatus{
				State:     "completed",
				Timestamp: &timestamp,
			},
		},
	}

	openAIReq := models.OpenAIRequest{Model: "gpt-4"}
	openAIResp := transformA2AToOpenAI(a2aResp, openAIReq)

	assert.Equal(t, "First part. Second part.", openAIResp.Choices[0].Message.Content)
}

func Test_transformA2AToOpenAI_FallbackToHistory(t *testing.T) {
	timestamp := "2025-10-02T12:00:00Z"
	a2aResp := models.SendMessageSuccessResponse{
		Jsonrpc: "2.0",
		Id:      1,
		Result: models.SendMessageSuccessResponseResult{
			Artifacts: []models.Artifact{}, // No artifacts
			ContextId: "context-123",
			History: []models.Message{
				{
					Kind:      "message",
					MessageId: "msg-1",
					Role:      "user",
					Parts: []models.MessagePartsElem{
						models.TextPart{Kind: "text", Text: "What is the weather?"},
					},
				},
				{
					Kind:      "message",
					MessageId: "msg-2",
					Role:      "agent",
					Parts: []models.MessagePartsElem{
						models.TextPart{Kind: "text", Text: "The weather is sunny."},
					},
				},
			},
			Id:        "task-123",
			Kind:      "task",
			MessageId: "msg-2",
			Role:      "agent",
			Parts: []models.MessagePartsElem{
				models.TextPart{Kind: "text", Text: "The weather is sunny."},
			},
			Status: models.TaskStatus{
				State:     "completed",
				Timestamp: &timestamp,
			},
		},
	}

	openAIReq := models.OpenAIRequest{Model: "gpt-4"}
	openAIResp := transformA2AToOpenAI(a2aResp, openAIReq)

	assert.Equal(t, "assistant", openAIResp.Choices[0].Message.Role)
	assert.Equal(t, "The weather is sunny.", openAIResp.Choices[0].Message.Content)
}

func Test_transformA2AToOpenAI_SkipsNonTextParts(t *testing.T) {
	timestamp := "2025-10-02T12:00:00Z"
	a2aResp := models.SendMessageSuccessResponse{
		Jsonrpc: "2.0",
		Id:      1,
		Result: models.SendMessageSuccessResponseResult{
			Artifacts: []models.Artifact{
				{
					ArtifactId: "artifact-123",
					Parts: []models.ArtifactPartsElem{
						models.DataPart{Kind: "data", Data: map[string]interface{}{"foo": "bar"}},
						models.TextPart{Kind: "text", Text: "Visible text"},
						models.FilePart{Kind: "file", File: models.FilePartFile{Uri: "base64data"}},
					},
				},
			},
			ContextId: "context-123",
			History:   []models.Message{},
			Id:        "task-123",
			Kind:      "task",
			MessageId: "msg-1",
			Role:      "agent",
			Parts: []models.MessagePartsElem{
				models.TextPart{Kind: "text", Text: "Visible text"},
			},
			Status: models.TaskStatus{
				State:     "completed",
				Timestamp: &timestamp,
			},
		},
	}

	openAIReq := models.OpenAIRequest{Model: "gpt-4"}
	openAIResp := transformA2AToOpenAI(a2aResp, openAIReq)

	assert.Equal(t, "Visible text", openAIResp.Choices[0].Message.Content)
}

// PAAL-223: Test that streaming requests are rejected with a clear error message
func TestStreamingRequestReturnsError(t *testing.T) {
	var extraConfig map[string]interface{}
	json.Unmarshal([]byte(configStr), &extraConfig)

	mockHandler := &MockHandler{}

	handlers, _ := HandlerRegisterer.registerHandlers(context.Background(), extraConfig, mockHandler)
	ts := httptest.NewUnstartedServer(handlers)
	ts.Start()
	defer ts.Close()

	openAIRequest := models.OpenAIRequest{
		Model: "gpt-4",
		Messages: []models.OpenAIMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: true, // Enable streaming
	}
	reqBody, _ := json.Marshal(openAIRequest)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/agent/chat/completions", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}

	resp, err := client.Do(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Verify error response format
	var errorResp map[string]interface{}
	respBody, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(respBody, &errorResp)
	assert.NoError(t, err)

	// Check error structure matches OpenAI format
	assert.Contains(t, errorResp, "error")
	errorObj := errorResp["error"].(map[string]interface{})
	assert.Equal(t, "Streaming is not currently supported by the Agent Gateway", errorObj["message"])
	assert.Equal(t, "invalid_request_error", errorObj["type"])

	// Verify backend was not called
	assert.Nil(t, mockHandler.ReceivedRequest)
}
