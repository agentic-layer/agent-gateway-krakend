package main

// AgentInfo represents an agent configuration
type AgentInfo struct {
	ModelID   string `json:"model_id"`
	URL       string `json:"url"`
	OwnedBy   string `json:"owned_by"`
	CreatedAt int64  `json:"createdAt"`
}

type config struct {
	Agents []AgentInfo `json:"agents"`
}