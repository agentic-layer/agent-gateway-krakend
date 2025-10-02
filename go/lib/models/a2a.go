package models

// A2A Protocol Request structures
type A2AMessagePart struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type A2AMessage struct {
	Role      string           `json:"role"`
	Parts     []A2AMessagePart `json:"parts"`
	MessageID string           `json:"messageId"`
	ContextID string           `json:"contextId"`
}

type A2AParams struct {
	Message  A2AMessage             `json:"message"`
	History  []A2AMessage           `json:"history,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type A2ARequest struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      int       `json:"id"`
	Method  string    `json:"method"`
	Params  A2AParams `json:"params"`
}

// A2A Response structures
type A2APart struct {
	Kind string `json:"kind"`
	Text string `json:"text,omitempty"`
	Data any    `json:"data,omitempty"`
}

type A2AArtifact struct {
	ArtifactID string    `json:"artifactId"`
	Parts      []A2APart `json:"parts"`
}

type A2AHistoryMessage struct {
	Kind      string    `json:"kind"`
	MessageID string    `json:"messageId"`
	Parts     []A2APart `json:"parts"`
	Role      string    `json:"role"`
	ContextID string    `json:"contextId,omitempty"`
	TaskID    string    `json:"taskId,omitempty"`
}

type A2AStatus struct {
	State     string `json:"state"`
	Timestamp string `json:"timestamp"`
}

type A2AResult struct {
	Artifacts []A2AArtifact       `json:"artifacts,omitempty"`
	ContextID string              `json:"contextId"`
	History   []A2AHistoryMessage `json:"history"`
	ID        string              `json:"id"`
	Kind      string              `json:"kind"`
	Metadata  map[string]any      `json:"metadata,omitempty"`
	Status    A2AStatus           `json:"status"`
}

type A2AResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      int       `json:"id"`
	Result  A2AResult `json:"result"`
}
