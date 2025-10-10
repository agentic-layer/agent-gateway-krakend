package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/agentic-layer/agent-gateway-krakend/lib/logging"
	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
	"github.com/go-http-utils/headers"
	"github.com/google/uuid"
)

const (
	pluginName      = "openai-a2a"
	configKey       = "openai_a2a_config"
	defaultEndpoint = "/chat/completions"
)

type config struct {
	Endpoint string `json:"endpoint"`
}

type registerer string

// HandlerRegisterer is the name of the symbol krakend looks up to try and register plugins
var HandlerRegisterer = registerer(pluginName)
var logger = logging.New(pluginName)

func main() {}

func init() {
	logger.Info("loaded")
}

func (r registerer) RegisterHandlers(f func(
	name string,
	handler func(context.Context, map[string]interface{}, http.Handler) (http.Handler, error),
)) {
	f(string(r), r.registerHandlers)
	logger.Info("registered")
}

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

func (r registerer) registerHandlers(_ context.Context, extra map[string]interface{}, handler http.Handler) (http.Handler, error) {
	var cfg config
	err := parseConfig(extra, &cfg)
	if err != nil {
		return nil, err
	}
	logger.Info("configuration loaded successfully")

	return http.HandlerFunc(r.handleRequest(cfg, handler)), nil
}

func (r registerer) handleRequest(cfg config, handler http.Handler) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		reqLogger := logging.NewWithPluginName(pluginName)

		// Check if this is a POST request to a chat completions endpoint
		if req.Method == http.MethodPost && isChatCompletionsEndpoint(req.URL.Path, cfg.Endpoint) {
			reqLogger.Info("intercepted OpenAI chat completions request: %s", req.URL.Path)

			// Extract the path prefix (agent name)
			pathPrefix := extractPathPrefix(req.URL.Path, cfg.Endpoint)
			if pathPrefix == "" {
				reqLogger.Error("unable to extract path prefix from: %s", req.URL.Path)
				http.Error(w, "invalid path format", http.StatusBadRequest)
				return
			}

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

			// Transform to A2A format
			a2aReq := transformOpenAIToA2A(openAIReq)

			// Marshal A2A request
			a2aBody, err := json.Marshal(a2aReq)
			if err != nil {
				reqLogger.Error("failed to marshal A2A request: %s", err)
				http.Error(w, "failed to create A2A request", http.StatusInternalServerError)
				return
			}

			reqLogger.Info("transformed OpenAI request to A2A format, forwarding to: /%s", pathPrefix)

			// Create new request to backend
			req.Body = io.NopCloser(bytes.NewReader(a2aBody))
			req.ContentLength = int64(len(a2aBody))
			req.URL.Path = "/" + pathPrefix
			req.Header.Set(headers.ContentType, "application/json")

			// Wrap response writer to capture A2A response
			rw := newResponseWriter(w)

			// Forward request to backend via KrakenD
			handler.ServeHTTP(rw, req)

			// Only transform successful responses
			if rw.statusCode != http.StatusOK {
				reqLogger.Info("backend returned non-OK status: %d, passing through", rw.statusCode)
				return
			}

			// Parse A2A response
			var a2aResp models.SendMessageSuccessResponse
			if err := json.Unmarshal(rw.body.Bytes(), &a2aResp); err != nil {
				reqLogger.Error("failed to parse A2A response: %s", err)
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

			reqLogger.Info("transformed A2A response back to OpenAI format")

			// Write the transformed response
			w.Header().Set(headers.ContentType, "application/json")
			// Remove Content-Length to allow for recalculation
			w.Header().Del(headers.ContentLength)
			w.WriteHeader(http.StatusOK)

			if _, err := w.Write(openAIRespBody); err != nil {
				reqLogger.Error("failed to write response: %s", err)
			}
			return
		}

		// Not a chat completions endpoint, pass through
		handler.ServeHTTP(w, req)
	}
}

// isChatCompletionsEndpoint checks if the path matches the chat completions pattern
func isChatCompletionsEndpoint(path string, endpoint string) bool {
	// Match pattern: /{path}/chat/completions or /{path}/chat/completion
	pattern := regexp.MustCompile(`^/[^/]+` + regexp.QuoteMeta(endpoint) + `$`)
	// Also support without leading slash in endpoint config
	if !strings.HasPrefix(endpoint, "/") {
		pattern = regexp.MustCompile(`^/[^/]+/` + regexp.QuoteMeta(endpoint) + `$`)
	}
	return pattern.MatchString(path)
}

// extractPathPrefix extracts the agent path prefix from the URL
// Example: /weather-agent/chat/completions -> weather-agent
func extractPathPrefix(path string, endpoint string) string {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Remove the endpoint suffix
	endpoint = strings.TrimPrefix(endpoint, "/")
	suffix := "/" + endpoint
	if strings.HasSuffix(path, suffix) {
		return strings.TrimSuffix(path, suffix)
	}

	return ""
}

// transformA2AToOpenAI converts A2A response to OpenAI chat completion format
func transformA2AToOpenAI(a2aResp models.SendMessageSuccessResponse, originalReq models.OpenAIRequest) models.OpenAIResponse {
	// Extract text from artifacts (preferred) or last agent message in history
	var content strings.Builder

	// First, try to get content from artifacts
	if len(a2aResp.Result.Artifacts) > 0 {
		for _, artifact := range a2aResp.Result.Artifacts {
			for _, part := range artifact.Parts {
				// Handle both concrete TextPart and map from JSON unmarshaling
				if textPart, ok := part.(models.TextPart); ok {
					content.WriteString(textPart.Text)
				} else if partMap, ok := part.(map[string]interface{}); ok {
					if kind, ok := partMap["kind"].(string); ok && kind == "text" {
						if text, ok := partMap["text"].(string); ok {
							content.WriteString(text)
						}
					}
				}
			}
		}
	}

	// If no artifacts, fall back to last agent message in history
	if content.Len() == 0 {
		for i := len(a2aResp.Result.History) - 1; i >= 0; i-- {
			msg := a2aResp.Result.History[i]
			if msg.Role == "agent" {
				for _, part := range msg.Parts {
					// Handle both concrete TextPart and map from JSON unmarshaling
					if textPart, ok := part.(models.TextPart); ok {
						content.WriteString(textPart.Text)
					} else if partMap, ok := part.(map[string]interface{}); ok {
						if kind, ok := partMap["kind"].(string); ok && kind == "text" {
							if text, ok := partMap["text"].(string); ok {
								content.WriteString(text)
							}
						}
					}
				}
				if content.Len() > 0 {
					break
				}
			}
		}
	}

	choice := models.OpenAIChoice{
		Index: 0,
		Message: struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			Role:    "assistant",
			Content: content.String(),
		},
		FinishReason: "stop",
	}

	return models.OpenAIResponse{
		ID:      uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   originalReq.Model,
		Choices: []models.OpenAIChoice{choice},
	}
}

// transformOpenAIToA2A converts OpenAI chat completion request to A2A format
func transformOpenAIToA2A(openAIReq models.OpenAIRequest) models.SendMessageRequest {
	contextID := uuid.New().String()
	messageID := uuid.New().String()

	// Get the last message (the current user message)
	lastMsg := openAIReq.Messages[len(openAIReq.Messages)-1]

	// Create the main message
	message := models.Message{
		Kind:      "message",
		MessageId: messageID,
		ContextId: &contextID,
		Role:      models.MessageRoleUser,
		Parts: []models.MessagePartsElem{
			models.TextPart{
				Kind: "text",
				Text: lastMsg.Content,
			},
		},
	}

	a2aReq := models.SendMessageRequest{
		Jsonrpc: "2.0",
		Id:      1,
		Method:  "message/send",
		Params: models.MessageSendParams{
			Message: message,
			Metadata: map[string]interface{}{
				"conversationId": contextID,
			},
		},
	}

	return a2aReq
}

func parseConfig(extra map[string]interface{}, config *config) error {
	if extra[configKey] == nil {
		config.Endpoint = defaultEndpoint
		logger.Info("using default %s.endpoint %v", configKey, defaultEndpoint)
		return nil
	}

	pluginConfig, ok := extra[configKey].(map[string]interface{})
	if !ok {
		return fmt.Errorf("cannot read extra_config.%s", configKey)
	}

	raw, err := json.Marshal(pluginConfig)
	if err != nil {
		return fmt.Errorf("cannot marshall extra config back to JSON: %s", err.Error())
	}
	err = json.Unmarshal(raw, config)
	if err != nil {
		return fmt.Errorf("cannot parse extra config: %s", err.Error())
	}

	if config.Endpoint == "" {
		config.Endpoint = defaultEndpoint
		logger.Info("using default %s.endpoint %v", configKey, defaultEndpoint)
	}

	return nil
}
