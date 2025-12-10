package main

import (
	"encoding/json"
	"net/http"

	"github.com/agentic-layer/agent-gateway-krakend/lib/logging"
)

// OpenAIModel represents a model in the OpenAI API format
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// OpenAIModelsResponse represents the response for /models endpoint
type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// handleModelsRequest handles GET /models requests by returning agents in OpenAI-compatible format.
// Agents are provided via plugin configuration from the operator.
func handleModelsRequest(w http.ResponseWriter, req *http.Request, agents []AgentInfo) {
	reqLogger := logging.NewWithPluginName(pluginName)

	if req.Method != http.MethodGet {
		reqLogger.Debug("invalid method for /models: %s", req.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reqLogger.Debug("handling /models request with %d configured agents", len(agents))

	// Build OpenAI models response - use namespace/name format
	models := make([]OpenAIModel, 0, len(agents))
	for _, agent := range agents {
		modelID := agent.Namespace + "/" + agent.Name

		models = append(models, OpenAIModel{
			ID:      modelID,
			Object:  "model",
			Created: agent.CreatedAt,
			OwnedBy: agent.Namespace,
		})
	}

	response := OpenAIModelsResponse{
		Object: "list",
		Data:   models,
	}

	// Marshal and send response
	responseBody, err := json.Marshal(response)
	if err != nil {
		reqLogger.Error("failed to marshal response: %s", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	reqLogger.Debug("returning %d models", len(models))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(responseBody); err != nil {
		reqLogger.Error("failed to write response: %s", err)
	}
}
