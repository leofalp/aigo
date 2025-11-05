package ai

import (
	"encoding/json"

	"github.com/leofalp/aigo/internal/jsonschema"
)

/*
	##### PROVIDER INPUT #####
*/

// ChatRequest represents a request to send a chat message
type ChatRequest struct {
	Model            string            `json:"model,omitempty"`              // Model name or identifier
	Messages         []Message         `json:"messages"`                     // Contains all messages in the conversation except system prompt
	SystemPrompt     string            `json:"system_prompt,omitempty"`      // Optional system prompt
	Tools            []ToolDescription `json:"tools,omitempty"`              // Contains tool definitions if any
	ResponseFormat   *ResponseFormat   `json:"response_format,omitempty"`    // Optional response format
	GenerationConfig *GenerationConfig `json:"generation_config,omitempty"`  // Optional generation configuration
	ToolChoiceForced string            `json:"tool_choice_forced,omitempty"` // Forced tool choice to not use computed one from tools.Required
}

type ToolDescription struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Parameters  *jsonschema.Schema `json:"parameters,omitempty"`
	Required    bool               `json:"required,omitempty"`
}

// Message represents a single message in a conversation
type Message struct {
	// Core fields (always present)
	Role    MessageRole `json:"role"`
	Content string      `json:"content,omitempty"`

	// Tool calling fields
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // For role=assistant requesting tools
	ToolCallID string     `json:"tool_call_id,omitempty"` // For role=tool, links to the tool call being responded to
	Name       string     `json:"name,omitempty"`         // For role=tool, name of the tool that generated this response

	// Extended fields
	Refusal   string `json:"refusal,omitempty"`   // If model refuses to respond (safety/policy)
	Reasoning string `json:"reasoning,omitempty"` // Chain-of-thought reasoning (o1/o3/gpt-5)

	// TODO support content types different than text in the future (images, audio, etc.)
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

	// Extended token metrics
	ReasoningTokens int `json:"reasoning_tokens,omitempty"` // Tokens used for reasoning (o1/o3/gpt-5)
	CachedTokens    int `json:"cached_tokens,omitempty"`    // Cached prompt tokens
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

	// Extended fields
	Refusal   string `json:"refusal,omitempty"`   // If model refuses to respond (safety/policy)
	Reasoning string `json:"reasoning,omitempty"` // Chain-of-thought reasoning (o1/o3/gpt-5)

	// TODO observability and debugging
	//HttpResponse *http.Response `json:"-"` // Raw HTTP response, if applicable
}

/*
	##### ENUMS #####
*/

// ToolCall represents a function/tool call request from the LLM
type ToolCall struct {
	ID       string           `json:"id,omitempty"` // Unique identifier for this tool call
	Type     string           `json:"type"`         // "function"
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolResult represents a standardized tool execution result.
// This structure provides consistent error handling and success reporting
// for tool executions, making it easier for LLMs to understand outcomes.
type ToolResult struct {
	Success bool        `json:"success"`           // Whether the tool executed successfully
	Error   string      `json:"error,omitempty"`   // Error type if success=false (e.g., "tool_not_found", "tool_execution_failed")
	Message string      `json:"message,omitempty"` // Human-readable message or error description
	Data    interface{} `json:"data,omitempty"`    // Actual result data if success=true
}

// NewToolResultSuccess creates a successful tool result.
// The data parameter contains the actual result from the tool execution.
func NewToolResultSuccess(data interface{}) ToolResult {
	return ToolResult{
		Success: true,
		Data:    data,
	}
}

// NewToolResultError creates a failed tool result with error details.
// errorType should be a machine-readable error code (e.g., "tool_not_found", "tool_execution_failed")
// message should be a human-readable description of what went wrong.
func NewToolResultError(errorType, message string) ToolResult {
	return ToolResult{
		Success: false,
		Error:   errorType,
		Message: message,
	}
}

// ToJSON converts the ToolResult to a JSON string.
// Returns the JSON string and any marshaling error.
func (tr ToolResult) ToJSON() (string, error) {
	bytes, err := json.Marshal(tr)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// MessageRole represents the role of a message; compatible with string
type MessageRole string

const (
	RoleSystem    MessageRole = "system"    // System instructions/configuration
	RoleUser      MessageRole = "user"      // End-user message
	RoleAssistant MessageRole = "assistant" // Middle llm response
	RoleTool      MessageRole = "tool"      // Tool/function output
)
