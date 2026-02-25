package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/agentic-layer/agent-gateway-krakend/lib/models"
)

// handleModelsRequest handles GET /models requests by returning agents in OpenAI-compatible format.
// Agents are provided via plugin configuration.
func handleModelsRequest(w http.ResponseWriter, req *http.Request, agents []AgentInfo) {
	if req.Method != http.MethodGet {
		logger.Debug("invalid method for /models:", req.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.Debug(fmt.Sprintf("handling /models request with %d configured agents", len(agents)))

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
		logger.Error("failed to marshal response:", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	logger.Debug(fmt.Sprintf("returning %d models", len(modelsList)))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(responseBody); err != nil {
		logger.Error("failed to write response:", err)
	}
}
