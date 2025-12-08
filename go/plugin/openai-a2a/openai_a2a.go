package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/agentic-layer/agent-gateway-krakend/lib/logging"
	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
	"github.com/google/uuid"
)

const (
	pluginName = "openai-a2a"
	configKey  = "openai_a2a_config"
)

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

func (r registerer) registerHandlers(_ context.Context, extra map[string]interface{}, handler http.Handler) (http.Handler, error) {
	var cfg config
	err := parseConfig(extra, &cfg)
	if err != nil {
		return nil, err
	}
	logger.Info("configuration loaded successfully with %d agents", len(cfg.Agents))

	return http.HandlerFunc(r.handleRequest(cfg, handler)), nil
}

func (r registerer) handleRequest(cfg config, handler http.Handler) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		// Handle GET /models endpoint
		if req.Method == http.MethodGet && req.URL.Path == "/models" {
			handleModelsRequest(w, req, cfg.Agents)
			return
		}

		// Handle POST /chat/completions endpoint (OpenAI-compatible)
		if req.Method == http.MethodPost && req.URL.Path == "/chat/completions" {
			handleGlobalChatCompletions(w, req, handler, cfg.Agents)
			return
		}

		// Pass through all other requests
		handler.ServeHTTP(w, req)
	}
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
func transformOpenAIToA2A(openAIReq models.OpenAIRequest, conversationId string) (*models.SendMessageRequest, error) {
	contextID := conversationId
	messageID := uuid.New().String()

	numMessages := len(openAIReq.Messages)
	if numMessages == 0 {
		return nil, errors.New("no messages found")
	}

	// Get the last message (the current user message)
	lastMsg := openAIReq.Messages[numMessages-1]

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
			Message:  message,
			Metadata: map[string]interface{}{},
		},
	}

	return &a2aReq, nil
}

func parseConfig(extra map[string]interface{}, config *config) error {
	if extra[configKey] == nil {
		// No config provided, use empty agents list
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

	return nil
}
