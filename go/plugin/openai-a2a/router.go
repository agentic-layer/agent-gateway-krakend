package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
	"github.com/go-http-utils/headers"
)

// responseWriter wraps http.ResponseWriter to capture response body
type responseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		statusCode:     http.StatusOK,
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.body.Write(b)
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
}

// ModelInfo contains routing information for an agent
type ModelInfo struct {
	ModelID string
	Path    string
	URL     string
}

// AgentResolutionError provides structured error information for agent resolution failures.
type AgentResolutionError struct {
	Type        string // "not_found", "configuration_error", "invalid_format"
	InternalMsg string // Detailed message for logging
	ClientMsg   string // Generic message for clients
}

func (e *AgentResolutionError) Error() string {
	return e.InternalMsg
}

// resolveAgentBackend resolves the agent backend URL from the model parameter.
func resolveAgentBackend(model string, agents []AgentInfo) (*ModelInfo, error) {
	if model == "" {
		return nil, &AgentResolutionError{
			Type:        "invalid_format",
			InternalMsg: "model parameter cannot be empty",
			ClientMsg:   "model parameter is required",
		}
	}

	// Check for invalid patterns
	if strings.Contains(model, "..") {
		return nil, &AgentResolutionError{
			Type:        "invalid_format",
			InternalMsg: fmt.Sprintf("invalid model parameter '%s': contains invalid pattern '..'", model),
			ClientMsg:   "invalid model parameter format",
		}
	}

	// Validate model parameter contains only valid URL path characters
	if strings.ContainsAny(model, "?#[]@!$&'()*+,;=") {
		return nil, &AgentResolutionError{
			Type:        "invalid_format",
			InternalMsg: fmt.Sprintf("invalid model parameter '%s': contains invalid characters", model),
			ClientMsg:   "invalid model parameter format",
		}
	}

	// Look for agent with matching model ID
	for _, agent := range agents {
		if agent.ModelID == model {
			if agent.URL == "" {
				return nil, &AgentResolutionError{
					Type:        "configuration_error",
					InternalMsg: fmt.Sprintf("agent %s has no URL configured", model),
					ClientMsg:   "model is not available",
				}
			}

			// Parse the URL to extract scheme and host
			parsedURL, err := url.Parse(agent.URL)
			if err != nil {
				return nil, &AgentResolutionError{
					Type:        "configuration_error",
					InternalMsg: fmt.Sprintf("failed to parse agent URL for %s: %v", model, err),
					ClientMsg:   "model is not available",
				}
			}

			backendURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

			// Construct routing path from model ID
			path := "/" + model

			return &ModelInfo{
				ModelID: model,
				Path:    path,
				URL:     backendURL,
			}, nil
		}
	}

	return nil, &AgentResolutionError{
		Type:        "not_found",
		InternalMsg: fmt.Sprintf("model %s not found in configuration", model),
		ClientMsg:   "model not found",
	}
}

// handleGlobalChatCompletions handles POST /chat/completions requests
func handleGlobalChatCompletions(w http.ResponseWriter, req *http.Request, handler http.Handler, agents []AgentInfo) {
	if req.Method != http.MethodPost {
		logger.Debug("invalid method for /chat/completions:", req.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read and parse OpenAI request
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Error("failed to read request body:", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	logger.Debug(fmt.Sprintf("handling global /chat/completions request:\n%s", string(bodyBytes)))

	var openAIReq models.OpenAIRequest
	if err := json.Unmarshal(bodyBytes, &openAIReq); err != nil {
		logger.Error("failed to parse OpenAI request:", err)
		http.Error(w, "invalid OpenAI request format", http.StatusBadRequest)
		return
	}

	// Check for streaming (not supported)
	if openAIReq.Stream {
		logger.Warning("streaming request detected, returning error (streaming not supported)")
		errorResponse := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Streaming is not currently supported by the Agent Gateway",
				"type":    "invalid_request_error",
				"code":    nil,
			},
		}
		w.Header().Set(headers.ContentType, "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
			logger.Error("failed to write response:", err)
		}
		return
	}

	// Check model parameter
	if openAIReq.Model == "" {
		logger.Error("model parameter is required")
		http.Error(w, "model parameter is required", http.StatusBadRequest)
		return
	}

	logger.Debug("resolving agent for model:", openAIReq.Model)

	// Resolve agent backend from config
	modelInfo, err := resolveAgentBackend(openAIReq.Model, agents)
	if err != nil {
		logger.Error("failed to resolve agent:", err)

		// Handle structured errors
		var resErr *AgentResolutionError
		if errors.As(err, &resErr) {
			statusCode := http.StatusBadRequest
			if resErr.Type == "not_found" {
				statusCode = http.StatusNotFound
			}
			http.Error(w, resErr.ClientMsg, statusCode)
		} else {
			// Fallback for unexpected errors
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	logger.Debug(fmt.Sprintf("resolved model %s with backend %s", modelInfo.ModelID, modelInfo.URL))

	// Get conversation ID from header
	conversationId := req.Header.Get("X-Conversation-ID")
	if conversationId == "" {
		logger.Warning("no X-Conversation-ID header found, generating new conversation ID")
		conversationId = fmt.Sprintf("%d", time.Now().UnixNano())
	} else {
		logger.Debug("using conversation ID from header:", conversationId)
	}

	// Transform to A2A format
	a2aReq, err := transformOpenAIToA2A(openAIReq, conversationId)
	if err != nil {
		logger.Error("failed to transform OpenAI request:", err)
		http.Error(w, "invalid OpenAI request", http.StatusBadRequest)
		return
	}

	// Marshal A2A request
	a2aBody, err := json.Marshal(a2aReq)
	if err != nil {
		logger.Error("failed to marshal A2A request:", err)
		http.Error(w, "failed to create A2A request", http.StatusInternalServerError)
		return
	}

	// Create new request to backend
	req.Body = io.NopCloser(bytes.NewReader(a2aBody))
	req.ContentLength = int64(len(a2aBody))
	req.URL.Path = modelInfo.Path
	req.Header.Set(headers.ContentType, "application/json")

	// Wrap response writer to capture A2A response
	rw := newResponseWriter(w)

	// Forward request to backend via KrakenD
	handler.ServeHTTP(rw, req)

	// Only transform successful responses
	if rw.statusCode != http.StatusOK {
		logger.Info(fmt.Sprintf("backend returned non-OK status: %d, passing through", rw.statusCode))
		// Copy headers from captured response
		for key, values := range rw.Header() {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(rw.statusCode)
		if _, err := w.Write(rw.body.Bytes()); err != nil {
			logger.Error("failed to write error response:", err)
		}
		return
	}

	// Parse A2A response
	var a2aResp models.SendMessageSuccessResponse
	a2aRespBytes := rw.body.Bytes()
	if err := json.Unmarshal(a2aRespBytes, &a2aResp); err != nil {
		logger.Error("failed to parse A2A response:", err)
		http.Error(w, "failed to parse backend response", http.StatusInternalServerError)
		return
	}

	// Transform A2A response back to OpenAI format
	openAIResp := transformA2AToOpenAI(a2aResp, openAIReq)

	// Marshal and send OpenAI response
	openAIRespBody, err := json.Marshal(openAIResp)
	if err != nil {
		logger.Error("failed to marshal OpenAI response:", err)
		http.Error(w, "failed to create OpenAI response", http.StatusInternalServerError)
		return
	}

	logger.Debug(fmt.Sprintf("transformed A2A response back to OpenAI format:\n%s", string(openAIRespBody)))

	// Write the transformed response
	w.Header().Set(headers.ContentType, "application/json")
	w.Header().Del(headers.ContentLength)
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(openAIRespBody); err != nil {
		logger.Error("failed to write response:", err)
	}
}
