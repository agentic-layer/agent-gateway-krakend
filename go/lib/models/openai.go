package models

// NOTE: These types are manually defined because the OpenAI OpenAPI spec cannot be generated
// due to OpenAPI 3.1.x compatibility issues with oapi-codegen.
// See: https://github.com/oapi-codegen/oapi-codegen/issues/373
// The OpenAI spec is available at: https://app.stainless.com/api/spec/documented/openai/openapi.documented.yml
//
// These types are simplified and only include the fields needed for basic chat completions.
// The full OpenAI API supports many additional fields (tools, functions, multi-modal content,
// streaming, advanced parameters, etc.) which are not included here.

// OpenAI Chat Completion Request structures
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// OpenAI Chat Completion Response structures
type OpenAIChoice struct {
	Index   int `json:"index"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
}

// OpenAI Models endpoint types
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}
