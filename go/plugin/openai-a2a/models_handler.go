package main

import (
	"encoding/json"
	"net/http"

	"github.com/agentic-layer/agent-gateway-krakend/lib/logging"
	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
)

// handleModelsRequest handles GET /models requests by returning agents in OpenAI-compatible format.
// Agents are provided via plugin configuration.
func handleModelsRequest(w http.ResponseWriter, req *http.Request, agents []AgentInfo) {
	reqLogger := logging.NewWithPluginName(pluginName)

	if req.Method != http.MethodGet {
		reqLogger.Debug("invalid method for /models: %s", req.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reqLogger.Debug("handling /models request with %d configured agents", len(agents))

	// Build OpenAI models response from configured agents
	modelsList := make([]models.OpenAIModel, 0, len(agents))
	for _, agent := range agents {
		modelsList = append(modelsList, models.OpenAIModel{
			ID:      agent.ModelID,
			Object:  "model",
			Created: agent.CreatedAt,
			OwnedBy: agent.OwnedBy,
		})
	}

	response := models.OpenAIModelsResponse{
		Object: "list",
		Data:   modelsList,
	}

	// Marshal and send response
	responseBody, err := json.Marshal(response)
	if err != nil {
		reqLogger.Error("failed to marshal response: %s", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	reqLogger.Debug("returning %d models", len(modelsList))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(responseBody); err != nil {
		reqLogger.Error("failed to write response: %s", err)
	}
}
