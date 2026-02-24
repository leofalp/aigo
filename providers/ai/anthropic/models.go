package anthropic

import "encoding/json"

/*
	ANTHROPIC MESSAGES API - REQUEST TYPES
*/

// anthropicRequest represents the request body for Anthropic's Messages API.
type anthropicRequest struct {
	Model        string                   `json:"model"`
	Messages     []anthropicMessage       `json:"messages"`
	System       json.RawMessage          `json:"system,omitempty"` // String or []anthropicContentBlock
	MaxTokens    int                      `json:"max_tokens"`       // Required by Anthropic on every request
	Temperature  *float64                 `json:"temperature,omitempty"`
	TopP         *float64                 `json:"top_p,omitempty"`
	TopK         *int                     `json:"top_k,omitempty"`
	Tools        []anthropicTool          `json:"tools,omitempty"`
	ToolChoice   *anthropicToolChoice     `json:"tool_choice,omitempty"`
	Stream       bool                     `json:"stream,omitempty"`
	Metadata     *anthropicMetadata       `json:"metadata,omitempty"`
	Thinking     *anthropicThinkingConfig `json:"thinking,omitempty"`
	OutputConfig *anthropicOutputConfig   `json:"output_config,omitempty"`
	Speed        string                   `json:"speed,omitempty"` // "fast" for research preview fast mode
}

// anthropicThinkingConfig controls extended/adaptive thinking on the request.
// For adaptive thinking (recommended for 4.6 models): Type="adaptive", BudgetTokens omitted.
// For manual thinking: Type="enabled", BudgetTokens set to a positive value.
type anthropicThinkingConfig struct {
	Type         string `json:"type"`                    // "adaptive" or "enabled"
	BudgetTokens int    `json:"budget_tokens,omitempty"` // Only for type="enabled"
}

// anthropicOutputConfig controls output effort level.
// Valid effort values: "low", "medium", "high", "max" (max is Opus 4.6 only).
type anthropicOutputConfig struct {
	Effort string `json:"effort"` // "low", "medium", "high", "max"
}

// anthropicMessage represents a single message in the conversation.
type anthropicMessage struct {
	Role    string                  `json:"role"`    // "user" or "assistant"
	Content []anthropicContentBlock `json:"content"` // Array of content blocks
}

// anthropicContentBlock is a discriminated union via the Type field.
// Depending on Type, different fields are populated:
//   - "text": Text + optional CacheControl
//   - "image": Source (base64 or url)
//   - "tool_use": ID, Name, Input
//   - "tool_result": ToolUseID, Content, IsError
//   - "thinking": Thinking, Signature
//   - "document": Source (base64 for PDF)
type anthropicContentBlock struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text,omitempty"`
	Source       *anthropicSource       `json:"source,omitempty"`        // For image and document types
	ID           string                 `json:"id,omitempty"`            // For tool_use
	Name         string                 `json:"name,omitempty"`          // For tool_use
	Input        json.RawMessage        `json:"input,omitempty"`         // For tool_use (arbitrary JSON)
	ToolUseID    string                 `json:"tool_use_id,omitempty"`   // For tool_result
	Content      json.RawMessage        `json:"content,omitempty"`       // For tool_result (string or content blocks)
	IsError      bool                   `json:"is_error,omitempty"`      // For tool_result
	Thinking     string                 `json:"thinking,omitempty"`      // For thinking blocks
	Signature    string                 `json:"signature,omitempty"`     // For thinking blocks (round-trip signature)
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"` // For prompt caching
}

// anthropicSource represents a media source (base64 inline or URL reference).
type anthropicSource struct {
	Type      string `json:"type"`                 // "base64" or "url"
	MediaType string `json:"media_type,omitempty"` // MIME type (for base64)
	Data      string `json:"data,omitempty"`       // Base64-encoded data
	URL       string `json:"url,omitempty"`        // URL reference
}

// anthropicCacheControl controls prompt caching on content blocks and tool definitions.
type anthropicCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// anthropicTool describes a tool/function available to the model.
type anthropicTool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	InputSchema  json.RawMessage        `json:"input_schema"`            // JSON Schema for tool input
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"` // For prompt caching on tool definitions
}

// anthropicToolChoice controls which tool the model should use.
type anthropicToolChoice struct {
	Type                   string `json:"type"`           // "auto", "any", "tool"
	Name                   string `json:"name,omitempty"` // Only for type="tool"
	DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
}

// anthropicMetadata contains optional request metadata.
type anthropicMetadata struct {
	UserID string `json:"user_id,omitempty"`
}

/*
	ANTHROPIC MESSAGES API - RESPONSE TYPES
*/

// anthropicResponse represents the response from Anthropic's Messages API.
type anthropicResponse struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`    // "message"
	Role         string                 `json:"role"`    // "assistant"
	Content      []responseContentBlock `json:"content"` // Response content blocks
	Model        string                 `json:"model"`
	StopReason   string                 `json:"stop_reason"`
	StopSequence string                 `json:"stop_sequence,omitempty"`
	Usage        anthropicUsage         `json:"usage"`
}

// responseContentBlock represents a content block in the response.
// The Type field discriminates between text, thinking, and tool_use blocks.
// Unknown type values are silently ignored during conversion for forward-compatibility.
type responseContentBlock struct {
	Type      string          `json:"type"`                // "text", "thinking", "tool_use"
	Text      string          `json:"text,omitempty"`      // For type="text"
	Thinking  string          `json:"thinking,omitempty"`  // For type="thinking"
	Signature string          `json:"signature,omitempty"` // For type="thinking" (round-trip)
	ID        string          `json:"id,omitempty"`        // For type="tool_use"
	Name      string          `json:"name,omitempty"`      // For type="tool_use"
	Input     json.RawMessage `json:"input,omitempty"`     // For type="tool_use" (arbitrary JSON)
}

// anthropicUsage reports token consumption for a single request.
type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}
