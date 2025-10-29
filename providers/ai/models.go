package ai

import (
	"aigo/internal/jsonschema"
)

/*
	##### PROVIDER INPUT #####
*/

// ChatRequest represents a request to send a chat message
type ChatRequest struct {
	Messages         []Message         `json:"messages"`                    // Contains all messages in the conversation except system prompt
	Model            string            `json:"model"`                       // Model identifier to use
	Models           []string          `json:"models,omitempty"`            // Optional list of model identifiers to use af fallback for model selection
	SystemPrompt     string            `json:"system_prompt,omitempty"`     // Optional system prompt
	Tools            []ToolDescription `json:"tools,omitempty"`             // Contains tool definitions if any
	ResponseFormat   *ResponseFormat   `json:"response_format,omitempty"`   // Optional response format
	GenerationConfig *GenerationConfig `json:"generation_config,omitempty"` // Optional generation configuration
	//ToolChoice      string            `json:"tool_choice,omitempty"` ->   // computed from tools.Required
}

type ToolDescription struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Parameters  *jsonschema.Schema `json:"parameters,omitempty"`
	Required    bool               `json:"required,omitempty"`
}

// Message represents a single message in a conversation
type Message struct {
	// Common fields
	Role    MessageRole `json:"role"`
	Content string      `json:"content"`
	// TODO support content types different than text in the future
}

type GenerationConfig struct {
	MaxTokens        int     `json:"max_tokens,omitempty"`        // Optional max tokens for the response
	Temperature      float32 `json:"temperature,omitempty"`       // Sampling temperature [0..2]. Higher => more random; lower => more deterministic.
	TopP             float32 `json:"top_p,omitempty"`             // OpenAi only: Nucleus (top-p) sampling [0..1]. Alternative to temperature; keeps tokens within top_p cumulative probability.
	FrequencyPenalty float32 `json:"frequency_penalty,omitempty"` // OpenAi only: Penalty [-2..2]. Positive values reduce repetition by penalizing frequent tokens.
	PresencePenalty  float32 `json:"presence_penalty,omitempty"`  // OpenAi only: Penalty [-2..2]. Positive values encourage new topics by penalizing tokens that already appeared.
	MaxOutputTokens  int     `json:"max_output_tokens,omitempty"` // Optional max tokens specifically for the output (if supported by provider)
}

type ResponseFormat struct {
	OutputSchema *jsonschema.Schema `json:"output_schema,omitempty"` // Optional schema for structured response. Implementation may vary by provider.
	Strict       bool               `json:"strict,omitempty"`        // If true, the model must strictly adhere to the output schema, if possible.
	Type         string             `json:"type,omitempty"`          // Optional type hint for the response format "text|json_object|json_schema|markdown|enum" - to use without schema, otherwise it will be forced to json_object
}

/*
	##### PROVIDER OUTPUT #####
*/

type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// ChatResponse represents the response from a chat completion
type ChatResponse struct {
	Id           string     `json:"id"`
	Model        string     `json:"model"`
	Object       string     `json:"object"`
	Created      int64      `json:"created"`
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
	Usage        *Usage     `json:"usage,omitempty"`

	// TODO observability and debugging
	//HttpResponse *http.Response `json:"-"` // Raw HTTP response, if applicable
}

/*
	##### ENUMS #####
*/

// ToolCall represents a function/tool call request from the LLM
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

// MessageRole represents the role of a message; compatible with string
type MessageRole string

const (
	RoleSystem    MessageRole = "system"    // System instructions/configuration
	RoleUser      MessageRole = "user"      // End-user message
	RoleAssistant MessageRole = "assistant" // Middle llm response
	RoleTool      MessageRole = "tool"      // Tool/function output
)
