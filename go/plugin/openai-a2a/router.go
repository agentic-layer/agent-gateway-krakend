package main

import (
	"bytes"
	"context"
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

// parseModelParameter parses and validates the model parameter
// Supports two formats:
//   - Simple: "agent-name" (namespace will be resolved from K8s)
//   - Namespaced: "namespace/agent-name"
//
// Returns an error if the model parameter doesn't conform to Kubernetes naming conventions
func parseModelParameter(model string) (namespace string, agentName string, isNamespaced bool, err error) {
	if model == "" {
		return "", "", false, fmt.Errorf("model parameter cannot be empty")
	}

	// Check for path traversal attempts
	if strings.Contains(model, "..") {
		return "", "", false, fmt.Errorf("invalid model parameter format")
	}

	if strings.Contains(model, "/") {
		parts := strings.Split(model, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", false, fmt.Errorf("invalid namespaced model format")
		}

		// Validate both namespace and agent name
		if err := validateK8sName(parts[0]); err != nil {
			return "", "", false, fmt.Errorf("invalid namespace: %w", err)
		}
		if err := validateK8sName(parts[1]); err != nil {
			return "", "", false, fmt.Errorf("invalid agent name: %w", err)
		}

		return parts[0], parts[1], true, nil
	}

	// Validate simple agent name
	if err := validateK8sName(model); err != nil {
		return "", "", false, fmt.Errorf("invalid agent name: %w", err)
	}

	return "", model, false, nil
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
	Type           string // "service_unavailable", "not_found", "ambiguous", "configuration_error"
	InternalMsg    string // Detailed message for logging
	ClientMsg      string // Generic message for clients
	ShouldRetry    bool
	RetryAfterSecs int
}

func (e *AgentResolutionError) Error() string {
	return e.InternalMsg
}

// resolveAgentBackend resolves the agent backend URL from the model parameter
// Returns the backend URL and agent name for routing
func resolveAgentBackend(ctx context.Context, model string) (*ModelInfo, error) {
	namespace, agentName, isNamespaced, err := parseModelParameter(model)
	if err != nil {
		return nil, &AgentResolutionError{
			Type:        "not_found",
			InternalMsg: fmt.Sprintf("invalid model parameter '%s': %v", model, err),
			ClientMsg:   "invalid model parameter format",
		}
	}

	// Create K8s client
	k8sClient, err := NewK8sClient()
	if err != nil {
		return nil, &AgentResolutionError{
			Type:           "service_unavailable",
			InternalMsg:    fmt.Sprintf("failed to create Kubernetes client: %v", err),
			ClientMsg:      "service temporarily unavailable",
			ShouldRetry:    true,
			RetryAfterSecs: 30,
		}
	}

	// List all exposed agents
	agents, err := k8sClient.ListExposedAgents(ctx)
	if err != nil {
		return nil, &AgentResolutionError{
			Type:           "service_unavailable",
			InternalMsg:    fmt.Sprintf("failed to list agents: %v", err),
			ClientMsg:      "service temporarily unavailable",
			ShouldRetry:    true,
			RetryAfterSecs: 30,
		}
	}

	if isNamespaced {
		// Look for agent with specific namespace
		for _, agent := range agents {
			if agent.Name == agentName && agent.Namespace == namespace {
				if agent.Status.URL == "" {
					return nil, &AgentResolutionError{
						Type:        "configuration_error",
						InternalMsg: fmt.Sprintf("agent %s/%s has no URL in status", namespace, agentName),
						ClientMsg:   "model is not available",
					}
				}

				// Parse the URL to get host
				parsedURL, err := url.Parse(agent.Status.URL)
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
			InternalMsg: fmt.Sprintf("agent %s/%s not found or not exposed", namespace, agentName),
			ClientMsg:   "model not found",
		}
	}

	// Simple format - find unique agent by name
	var matchingAgents []Agent
	for _, agent := range agents {
		if agent.Name == agentName {
			matchingAgents = append(matchingAgents, agent)
		}
	}

	if len(matchingAgents) == 0 {
		return nil, &AgentResolutionError{
			Type:        "not_found",
			InternalMsg: fmt.Sprintf("agent %s not found or not exposed", agentName),
			ClientMsg:   "model not found",
		}
	}

	if len(matchingAgents) > 1 {
		// Build list of available namespaced names for logging
		var namespaces []string
		for _, agent := range matchingAgents {
			namespaces = append(namespaces, fmt.Sprintf("%s/%s", agent.Namespace, agent.Name))
		}
		return nil, &AgentResolutionError{
			Type: "ambiguous",
			InternalMsg: fmt.Sprintf("multiple agents named %s found in namespaces: %s",
				agentName, strings.Join(namespaces, ", ")),
			ClientMsg: "model name is ambiguous, please use the format 'namespace/model'",
		}
	}

	// Single match found
	agent := matchingAgents[0]
	if agent.Status.URL == "" {
		return nil, &AgentResolutionError{
			Type:        "configuration_error",
			InternalMsg: fmt.Sprintf("agent %s in namespace %s has no URL in status", agentName, agent.Namespace),
			ClientMsg:   "model is not available",
		}
	}

	// Parse the URL to get host
	parsedURL, err := url.Parse(agent.Status.URL)
	if err != nil {
		return nil, &AgentResolutionError{
			Type:        "configuration_error",
			InternalMsg: fmt.Sprintf("failed to parse agent URL for %s: %v", agentName, err),
			ClientMsg:   "model is not available",
		}
	}

	backendURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	return &ModelInfo{
		Name:      agent.Name,
		Namespace: agent.Namespace,
		URL:       backendURL,
	}, nil
}

// handleGlobalChatCompletions handles POST /chat/completions requests
func handleGlobalChatCompletions(w http.ResponseWriter, req *http.Request, handler http.Handler) {
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

	// Check for streaming (not supported) - check early to avoid unnecessary K8s lookups
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

	// Resolve agent backend
	modelInfo, err := resolveAgentBackend(req.Context(), openAIReq.Model)
	if err != nil {
		reqLogger.Error("failed to resolve agent: %s", err)

		// Handle structured errors
		if resErr, ok := err.(*AgentResolutionError); ok {
			if resErr.ShouldRetry {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", resErr.RetryAfterSecs))
				http.Error(w, resErr.ClientMsg, http.StatusServiceUnavailable)
			} else {
				statusCode := http.StatusBadRequest
				if resErr.Type == "not_found" {
					statusCode = http.StatusNotFound
				}
				http.Error(w, resErr.ClientMsg, statusCode)
			}
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

	// Route to namespaced endpoint (always uses namespace prefix for reliability)
	// The operator generates both /namespace/agent and /agent endpoints for unique agents,
	// but we always route via namespaced format to avoid ambiguity
	namespacedPath := "/" + modelInfo.Namespace + "/" + modelInfo.Name
	reqLogger.Debug("transformed OpenAI request to A2A format, forwarding to: %s", namespacedPath)

	// Create new request to backend
	req.Body = io.NopCloser(bytes.NewReader(a2aBody))
	req.ContentLength = int64(len(a2aBody))
	req.URL.Path = namespacedPath
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
