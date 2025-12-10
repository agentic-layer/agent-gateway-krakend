package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/agentic-layer/agent-gateway-krakend/lib/logging"
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

// ModelInfo contains information about an agent model
type ModelInfo struct {
	Name      string
	Namespace string
	URL       string
}

// parseModelParameter parses and validates the model parameter.
// Requires format: "namespace/agent-name"
// Returns an error if the model parameter doesn't conform to the expected format.
func parseModelParameter(model string) (namespace string, agentName string, err error) {
	if model == "" {
		return "", "", fmt.Errorf("model parameter cannot be empty")
	}

	// Check for path traversal attempts
	if strings.Contains(model, "..") {
		return "", "", fmt.Errorf("invalid model parameter format")
	}

	// Requires format: namespace/agent-name
	if !strings.Contains(model, "/") {
		return "", "", fmt.Errorf("model parameter must be in format 'namespace/agent-name'")
	}

	parts := strings.Split(model, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid model format, expected 'namespace/agent-name'")
	}

	// Validate both namespace and agent name
	if err := validateK8sName(parts[0]); err != nil {
		return "", "", fmt.Errorf("invalid namespace: %w", err)
	}
	if err := validateK8sName(parts[1]); err != nil {
		return "", "", fmt.Errorf("invalid agent name: %w", err)
	}

	return parts[0], parts[1], nil
}

// validateK8sName validates that a string conforms to Kubernetes DNS subdomain name rules
// Names must:
// - contain only lowercase alphanumeric characters, '-' or '.'
// - start and end with an alphanumeric character
// - be at most 253 characters long
func validateK8sName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if len(name) > 253 {
		return fmt.Errorf("name exceeds maximum length of 253 characters")
	}

	// Check first character (must be alphanumeric)
	if !isAlphanumeric(rune(name[0])) {
		return fmt.Errorf("name must start with an alphanumeric character")
	}

	// Check last character (must be alphanumeric)
	if !isAlphanumeric(rune(name[len(name)-1])) {
		return fmt.Errorf("name must end with an alphanumeric character")
	}

	// Check all characters
	for _, char := range name {
		if !isAlphanumeric(char) && char != '-' && char != '.' {
			return fmt.Errorf("name contains invalid character '%c'", char)
		}
	}

	return nil
}

// isAlphanumeric checks if a rune is a lowercase letter or digit
func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
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

// resolveAgentBackend resolves the agent backend URL from the model parameter using config-provided agents.
// Requires model names in format "namespace/agent-name".
func resolveAgentBackend(model string, agents []AgentInfo) (*ModelInfo, error) {
	namespace, agentName, err := parseModelParameter(model)
	if err != nil {
		return nil, &AgentResolutionError{
			Type:        "invalid_format",
			InternalMsg: fmt.Sprintf("invalid model parameter '%s': %v", model, err),
			ClientMsg:   fmt.Sprintf("invalid model format: %v", err),
		}
	}

	// Look for agent with matching namespace and name
	for _, agent := range agents {
		if agent.Name == agentName && agent.Namespace == namespace {
			if agent.URL == "" {
				return nil, &AgentResolutionError{
					Type:        "configuration_error",
					InternalMsg: fmt.Sprintf("agent %s/%s has no URL configured", namespace, agentName),
					ClientMsg:   "model is not available",
				}
			}

			// Parse the URL to get host
			parsedURL, err := url.Parse(agent.URL)
			if err != nil {
				return nil, &AgentResolutionError{
					Type:        "configuration_error",
					InternalMsg: fmt.Sprintf("failed to parse agent URL for %s/%s: %v", namespace, agentName, err),
					ClientMsg:   "model is not available",
				}
			}

			backendURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

			return &ModelInfo{
				Name:      agentName,
				Namespace: namespace,
				URL:       backendURL,
			}, nil
		}
	}

	return nil, &AgentResolutionError{
		Type:        "not_found",
		InternalMsg: fmt.Sprintf("agent %s/%s not found in configuration", namespace, agentName),
		ClientMsg:   "model not found",
	}
}

// handleGlobalChatCompletions handles POST /chat/completions requests
func handleGlobalChatCompletions(w http.ResponseWriter, req *http.Request, handler http.Handler, agents []AgentInfo) {
	reqLogger := logging.NewWithPluginName(pluginName)

	if req.Method != http.MethodPost {
		reqLogger.Debug("invalid method for /chat/completions: %s", req.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reqLogger.Debug("handling global /chat/completions request")

	// Read and parse OpenAI request
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		reqLogger.Error("failed to read request body: %s", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	var openAIReq models.OpenAIRequest
	if err := json.Unmarshal(bodyBytes, &openAIReq); err != nil {
		reqLogger.Error("failed to parse OpenAI request: %s", err)
		http.Error(w, "invalid OpenAI request format", http.StatusBadRequest)
		return
	}

	// Check for streaming (not supported)
	if openAIReq.Stream {
		reqLogger.Warn("streaming request detected, returning error (streaming not supported)")
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
			reqLogger.Error("failed to write response: %s", err)
		}
		return
	}

	// Check model parameter
	if openAIReq.Model == "" {
		reqLogger.Error("model parameter is required")
		http.Error(w, "model parameter is required", http.StatusBadRequest)
		return
	}

	reqLogger.Debug("resolving agent for model: %s", openAIReq.Model)

	// Resolve agent backend from config
	modelInfo, err := resolveAgentBackend(openAIReq.Model, agents)
	if err != nil {
		reqLogger.Error("failed to resolve agent: %s", err)

		// Handle structured errors
		if resErr, ok := err.(*AgentResolutionError); ok {
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

	reqLogger.Debug("resolved agent %s in namespace %s with backend %s", modelInfo.Name, modelInfo.Namespace, modelInfo.URL)

	// Get conversation ID from header
	conversationId := req.Header.Get("X-Conversation-ID")
	if conversationId == "" {
		reqLogger.Warn("no X-Conversation-ID header found, generating new conversation ID")
		conversationId = fmt.Sprintf("%d", time.Now().UnixNano())
	} else {
		reqLogger.Debug("using conversation ID from header: %s", conversationId)
	}

	// Transform to A2A format
	a2aReq, err := transformOpenAIToA2A(openAIReq, conversationId)
	if err != nil {
		reqLogger.Error("failed to transform OpenAI request: %s", err)
		http.Error(w, "invalid OpenAI request", http.StatusBadRequest)
		return
	}

	// Marshal A2A request
	a2aBody, err := json.Marshal(a2aReq)
	if err != nil {
		reqLogger.Error("failed to marshal A2A request: %s", err)
		http.Error(w, "failed to create A2A request", http.StatusInternalServerError)
		return
	}

	// Route to agent endpoint
	agentPath := "/" + modelInfo.Namespace + "/" + modelInfo.Name
	reqLogger.Debug("transformed OpenAI request to A2A format, forwarding to: %s", agentPath)

	// Create new request to backend
	req.Body = io.NopCloser(bytes.NewReader(a2aBody))
	req.ContentLength = int64(len(a2aBody))
	req.URL.Path = agentPath
	req.Header.Set(headers.ContentType, "application/json")

	// Wrap response writer to capture A2A response
	rw := newResponseWriter(w)

	// Forward request to backend via KrakenD
	handler.ServeHTTP(rw, req)

	// Only transform successful responses
	if rw.statusCode != http.StatusOK {
		reqLogger.Info("backend returned non-OK status: %d, passing through", rw.statusCode)
		// Copy headers from captured response
		for key, values := range rw.Header() {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(rw.statusCode)
		if _, err := w.Write(rw.body.Bytes()); err != nil {
			reqLogger.Error("failed to write error response: %s", err)
		}
		return
	}

	// Parse A2A response
	var a2aResp models.SendMessageSuccessResponse
	if err := json.Unmarshal(rw.body.Bytes(), &a2aResp); err != nil {
		reqLogger.Error("failed to parse A2A response: %s", err)
		http.Error(w, "failed to parse backend response", http.StatusInternalServerError)
		return
	}

	// Transform A2A response back to OpenAI format
	openAIResp := transformA2AToOpenAI(a2aResp, openAIReq)

	// Marshal and send OpenAI response
	openAIRespBody, err := json.Marshal(openAIResp)
	if err != nil {
		reqLogger.Error("failed to marshal OpenAI response: %s", err)
		http.Error(w, "failed to create OpenAI response", http.StatusInternalServerError)
		return
	}

	reqLogger.Debug("transformed A2A response back to OpenAI format")

	// Write the transformed response
	w.Header().Set(headers.ContentType, "application/json")
	w.Header().Del(headers.ContentLength)
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(openAIRespBody); err != nil {
		reqLogger.Error("failed to write response: %s", err)
	}
}
