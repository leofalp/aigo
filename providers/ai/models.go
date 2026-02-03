package ai

import (
	"encoding/json"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/internal/jsonschema"
)

/*
	##### PROVIDER INPUT #####
*/

// ChatRequest represents a request to send a chat message
type ChatRequest struct {
	Model            string            `json:"model,omitempty"`         // Model name or identifier
	Messages         []Message         `json:"messages"`                // Contains all messages in the conversation except system prompt
	SystemPrompt     string            `json:"system_prompt,omitempty"` // Optional system prompt
	Tools            []ToolDescription `json:"tools,omitempty"`         // Contains tool definitions if any
	ToolChoice       *ToolChoice       `json:"tool_choice,omitempty"`
	ResponseFormat   *ResponseFormat   `json:"response_format,omitempty"`   // Optional response format
	GenerationConfig *GenerationConfig `json:"generation_config,omitempty"` // Optional generation configuration
}

type ToolChoice struct {
	ToolChoiceForced   string             `json:"tool_choice_forced,omitempty"`    // Forced tool choice to not use computed one from tools.Required
	AtLeastOneRequired bool               `json:"at_least_one_required,omitempty"` // If true, at least one tool from ChatRequest.Tools must be used
	RequiredTools      []*ToolDescription `json:"required_tools,omitempty"`        // List of required tool (must be declared in ChatRequest.Tools)
}

type ToolDescription struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Parameters  *jsonschema.Schema `json:"parameters,omitempty"`
	// Metrics contains optional metadata about tool execution cost and performance.
	// This is not sent to the LLM provider unless IncludeCostInDescription is used.
	Metrics *cost.ToolMetrics `json:"metrics,omitempty"`
}

// Built-in tool names for provider-specific capabilities.
// These are "pseudo-tools" that enable special provider features.
// Currently supported by: Gemini. Other providers may add support or ignore them.
// Prefix with underscore to distinguish from user-defined tools.
const (
	ToolGoogleSearch  = "_google_search"  // Web search grounding (Gemini)
	ToolURLContext    = "_url_context"    // URL content grounding (Gemini)
	ToolCodeExecution = "_code_execution" // Code execution sandbox (Gemini)
)

// IsBuiltinTool returns true if the tool name is a built-in pseudo-tool.
// Providers should check this to handle built-in tools differently from user tools.
func IsBuiltinTool(name string) bool {
	return len(name) > 0 && name[0] == '_'
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

	// Extended thinking/reasoning configuration.
	// Currently supported by: Gemini (thinkingBudget)
	// Providers that don't support these fields will ignore them.
	ThinkingBudget  *int `json:"thinking_budget,omitempty"`  // Token budget for reasoning (0=disable, -1=dynamic)
	IncludeThoughts bool `json:"include_thoughts,omitempty"` // Include reasoning in response

	// Safety/content filtering configuration.
	// Currently supported by: Gemini.
	// Other providers may use their own safety mechanisms or ignore this field.
	SafetySettings []SafetySetting `json:"safety_settings,omitempty"`
}

// SafetySetting configures content safety thresholds.
// Provider-agnostic structure that can be extended for future providers.
type SafetySetting struct {
	Category  string `json:"category"`  // Category identifier (provider-specific)
	Threshold string `json:"threshold"` // Threshold level (provider-specific)
}

// Gemini-specific safety categories. Other providers may define their own.
const (
	HarmCategoryHarassment       = "HARM_CATEGORY_HARASSMENT"
	HarmCategoryHateSpeech       = "HARM_CATEGORY_HATE_SPEECH"
	HarmCategorySexuallyExplicit = "HARM_CATEGORY_SEXUALLY_EXPLICIT"
	HarmCategoryDangerousContent = "HARM_CATEGORY_DANGEROUS_CONTENT"
)

// Gemini-specific safety thresholds. Other providers may define their own.
const (
	BlockNone           = "BLOCK_NONE"
	BlockOnlyHigh       = "BLOCK_ONLY_HIGH"
	BlockMediumAndAbove = "BLOCK_MEDIUM_AND_ABOVE"
	BlockLowAndAbove    = "BLOCK_LOW_AND_ABOVE"
)

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

	// Grounding contains citation and source attribution (web search, RAG, etc.)
	Grounding *GroundingMetadata `json:"grounding,omitempty"`

	// TODO observability and debugging
	//HttpResponse *http.Response `json:"-"` // Raw HTTP response, if applicable
}

// GroundingMetadata contains citation and source attribution from grounded responses.
// This structure is provider-agnostic and supports Gemini, OpenAI, and Anthropic.
type GroundingMetadata struct {
	// Sources is the list of source documents/URLs used for grounding.
	Sources []GroundingSource `json:"sources,omitempty"`

	// Citations links specific text segments to their supporting sources.
	Citations []Citation `json:"citations,omitempty"`

	// SearchQueries contains the search queries used (Gemini-specific).
	SearchQueries []string `json:"search_queries,omitempty"`
}

// GroundingSource represents a source document or URL.
type GroundingSource struct {
	Index int    `json:"index"` // 0-based index for reference from Citations
	URI   string `json:"uri"`
	Title string `json:"title,omitempty"`
}

// Citation links a text segment to its supporting sources.
type Citation struct {
	Text          string    `json:"text,omitempty"`       // The cited text (optional)
	StartIndex    int       `json:"start_index"`          // Character start (0-indexed)
	EndIndex      int       `json:"end_index"`            // Character end (0-indexed, exclusive)
	SourceIndices []int     `json:"source_indices"`       // References to Sources array
	Confidence    []float64 `json:"confidence,omitempty"` // Confidence scores (optional)
}

// StructuredChatResponse wraps a ChatResponse with parsed structured data.
// This type is returned by StructuredClient to provide both the parsed data
// and access to the raw response for metadata like usage and reasoning.
type StructuredChatResponse[T any] struct {
	ChatResponse    // Raw response with metadata (usage, reasoning, etc.)
	Data         *T // Parsed structured data
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
