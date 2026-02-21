// Package ai defines the shared types and interfaces used across all AI providers
// (OpenAI, Gemini, Anthropic). Types in this package are provider-agnostic: each
// provider's conversion layer is responsible for mapping them to wire format.
package ai

import (
	"encoding/json"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/internal/jsonschema"
)

/*
	##### PROVIDER INPUT #####
*/

// ChatRequest represents a single chat completion request sent to a provider.
// Messages contains the full conversation history (excluding the system prompt).
// Optional fields such as Tools, ToolChoice, ResponseFormat, and GenerationConfig
// are forwarded to the provider only when non-nil / non-empty.
type ChatRequest struct {
	Model            string            `json:"model,omitempty"`         // Model name or identifier
	Messages         []Message         `json:"messages"`                // Contains all messages in the conversation except system prompt
	SystemPrompt     string            `json:"system_prompt,omitempty"` // Optional system prompt
	Tools            []ToolDescription `json:"tools,omitempty"`         // Contains tool definitions if any
	ToolChoice       *ToolChoice       `json:"tool_choice,omitempty"`
	ResponseFormat   *ResponseFormat   `json:"response_format,omitempty"`   // Optional response format
	GenerationConfig *GenerationConfig `json:"generation_config,omitempty"` // Optional generation configuration
}

// ToolChoice controls which tool(s) the model is allowed or required to call.
// When ToolChoiceForced is set it overrides any automatic selection derived from
// RequiredTools. AtLeastOneRequired ensures the model calls at least one tool
// declared in ChatRequest.Tools.
type ToolChoice struct {
	ToolChoiceForced   string             `json:"tool_choice_forced,omitempty"`    // Forced tool choice to not use computed one from tools.Required
	AtLeastOneRequired bool               `json:"at_least_one_required,omitempty"` // If true, at least one tool from ChatRequest.Tools must be used
	RequiredTools      []*ToolDescription `json:"required_tools,omitempty"`        // List of required tool (must be declared in ChatRequest.Tools)
}

// ToolDescription describes a function that the model may call during a
// conversation. Name and Description are sent verbatim to the provider;
// Parameters defines the expected JSON schema for arguments. Metrics carries
// optional cost/performance metadata for the orchestration layer only and is
// never forwarded to the provider.
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

// ContentType represents the type of a content part in a multimodal message.
type ContentType string

const (
	ContentTypeText     ContentType = "text"     // Plain text content
	ContentTypeImage    ContentType = "image"    // Image content (base64-encoded or URI reference)
	ContentTypeAudio    ContentType = "audio"    // Audio content (base64-encoded or URI reference)
	ContentTypeVideo    ContentType = "video"    // Video content (base64-encoded or URI reference)
	ContentTypeDocument ContentType = "document" // Document content (PDF, plain text; base64-encoded or URI reference)
)

// ContentPart represents a single part of a multimodal message.
// A message can contain multiple parts mixing text, images, audio, video, and documents.
// Each provider's conversion layer maps these parts to the appropriate wire format.
type ContentPart struct {
	Type     ContentType   `json:"type"`
	Text     string        `json:"text,omitempty"`
	Image    *ImageData    `json:"image,omitempty"`
	Audio    *AudioData    `json:"audio,omitempty"`
	Video    *VideoData    `json:"video,omitempty"`
	Document *DocumentData `json:"document,omitempty"`
}

// ImageData holds image content, either as base64-encoded inline data or a URI reference.
// Exactly one of Data or URI should be set. Each provider decides the wire format:
//   - Gemini: URI maps to fileData, Data maps to inlineData
//   - OpenAI: URI maps to image_url, Data maps to base64 data URL
//   - Anthropic: URI maps to url source, Data maps to base64 source
type ImageData struct {
	MimeType string `json:"mime_type"`      // MIME type (e.g., "image/png", "image/jpeg")
	Data     string `json:"data,omitempty"` // Base64-encoded image data
	URI      string `json:"uri,omitempty"`  // URL, file URI, or opaque file ID
}

// AudioData holds audio content, either as base64-encoded inline data or a URI reference.
// Exactly one of Data or URI should be set.
// MimeType uses the canonical MIME form (e.g., "audio/wav", "audio/mp3").
// Providers that require format strings (e.g., OpenAI uses "wav" instead of "audio/wav")
// handle the conversion internally.
type AudioData struct {
	MimeType string `json:"mime_type"`      // MIME type (e.g., "audio/wav", "audio/mp3", "audio/ogg")
	Data     string `json:"data,omitempty"` // Base64-encoded audio data
	URI      string `json:"uri,omitempty"`  // URL, file URI, or opaque file ID
}

// VideoData holds video content, either as base64-encoded inline data or a URI reference.
// Exactly one of Data or URI should be set.
type VideoData struct {
	MimeType string `json:"mime_type"`      // MIME type (e.g., "video/mp4", "video/webm")
	Data     string `json:"data,omitempty"` // Base64-encoded video data
	URI      string `json:"uri,omitempty"`  // URL, file URI, or opaque file ID
}

// DocumentData holds document content, either as base64-encoded inline data or a URI reference.
// Exactly one of Data or URI should be set.
// Supported formats depend on the provider (e.g., Gemini supports PDF via inline or file URI).
type DocumentData struct {
	MimeType string `json:"mime_type"`      // MIME type (e.g., "application/pdf", "text/plain")
	Data     string `json:"data,omitempty"` // Base64-encoded document data
	URI      string `json:"uri,omitempty"`  // URL, file URI, or opaque file ID
}

// NewTextPart creates a ContentPart containing text content.
func NewTextPart(text string) ContentPart {
	return ContentPart{
		Type: ContentTypeText,
		Text: text,
	}
}

// NewImagePart creates a ContentPart containing base64-encoded image data.
func NewImagePart(mimeType, base64Data string) ContentPart {
	return ContentPart{
		Type: ContentTypeImage,
		Image: &ImageData{
			MimeType: mimeType,
			Data:     base64Data,
		},
	}
}

// NewImagePartFromURI creates a ContentPart referencing an image by URL or file URI.
// The provider's conversion layer determines the wire format (e.g., Gemini fileData, OpenAI image_url).
func NewImagePartFromURI(mimeType, uri string) ContentPart {
	return ContentPart{
		Type: ContentTypeImage,
		Image: &ImageData{
			MimeType: mimeType,
			URI:      uri,
		},
	}
}

// NewAudioPart creates a ContentPart containing base64-encoded audio data.
// mimeType should be the canonical MIME form (e.g., "audio/wav", "audio/mp3").
func NewAudioPart(mimeType, base64Data string) ContentPart {
	return ContentPart{
		Type: ContentTypeAudio,
		Audio: &AudioData{
			MimeType: mimeType,
			Data:     base64Data,
		},
	}
}

// NewAudioPartFromURI creates a ContentPart referencing audio by URL or file URI.
func NewAudioPartFromURI(mimeType, uri string) ContentPart {
	return ContentPart{
		Type: ContentTypeAudio,
		Audio: &AudioData{
			MimeType: mimeType,
			URI:      uri,
		},
	}
}

// NewVideoPart creates a ContentPart containing base64-encoded video data.
func NewVideoPart(mimeType, base64Data string) ContentPart {
	return ContentPart{
		Type: ContentTypeVideo,
		Video: &VideoData{
			MimeType: mimeType,
			Data:     base64Data,
		},
	}
}

// NewVideoPartFromURI creates a ContentPart referencing video by URL or file URI.
func NewVideoPartFromURI(mimeType, uri string) ContentPart {
	return ContentPart{
		Type: ContentTypeVideo,
		Video: &VideoData{
			MimeType: mimeType,
			URI:      uri,
		},
	}
}

// NewDocumentPart creates a ContentPart containing base64-encoded document data (e.g., PDF).
func NewDocumentPart(mimeType, base64Data string) ContentPart {
	return ContentPart{
		Type: ContentTypeDocument,
		Document: &DocumentData{
			MimeType: mimeType,
			Data:     base64Data,
		},
	}
}

// NewDocumentPartFromURI creates a ContentPart referencing a document by URL or file URI.
func NewDocumentPartFromURI(mimeType, uri string) ContentPart {
	return ContentPart{
		Type: ContentTypeDocument,
		Document: &DocumentData{
			MimeType: mimeType,
			URI:      uri,
		},
	}
}

// Message represents a single turn in a conversation. The Role field
// determines how the provider interprets the content. When ContentParts is
// populated it takes precedence over the plain-text Content field, enabling
// multimodal messages. Tool-related fields (ToolCalls, ToolCallID, Name) are
// only relevant for assistant and tool roles respectively.
type Message struct {
	// Core fields (always present)
	Role    MessageRole `json:"role"`
	Content string      `json:"content,omitempty"`

	// Multimodal content parts (optional, takes precedence over Content when populated)
	ContentParts []ContentPart `json:"content_parts,omitempty"`

	// Tool calling fields
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // For role=assistant requesting tools
	ToolCallID string     `json:"tool_call_id,omitempty"` // For role=tool, links to the tool call being responded to
	Name       string     `json:"name,omitempty"`         // For role=tool, name of the tool that generated this response

	// Code execution results from server-side sandbox execution (Gemini code_execution tool).
	// Present on role=assistant messages when the model generated and executed code.
	// Used for multi-turn round-tripping: providers serialize these back to their wire format.
	CodeExecutions []CodeExecution `json:"code_executions,omitempty"`

	// Extended fields
	Refusal   string `json:"refusal,omitempty"`   // If model refuses to respond (safety/policy)
	Reasoning string `json:"reasoning,omitempty"` // Chain-of-thought reasoning (o1/o3/gpt-5)
}

// GenerationConfig holds sampling and output-control parameters sent to the
// provider with each request. Fields that are unsupported by a given provider
// are silently ignored by that provider's conversion layer.
// Zero values are treated as "not set" (providers use their own defaults).
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

	// ResponseModalities specifies the desired output modalities (e.g., ["TEXT", "IMAGE"]).
	// Currently supported by: Gemini (for image generation models).
	ResponseModalities []string `json:"response_modalities,omitempty"`
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

// ResponseFormat instructs the provider to emit output in a specific structure.
// When OutputSchema is set the provider is asked to produce JSON conforming to
// that schema. The Type hint selects a named format preset (e.g., "json_object",
// "json_schema") when no explicit schema is provided. Strict mode, when
// supported, causes the provider to enforce the schema without fallback.
type ResponseFormat struct {
	OutputSchema *jsonschema.Schema `json:"output_schema,omitempty"` // Optional schema for structured response. Implementation may vary by provider.
	Strict       bool               `json:"strict,omitempty"`        // If true, the model must strictly adhere to the output schema, if possible.
	Type         string             `json:"type,omitempty"`          // Optional type hint for the response format "text|json_object|json_schema|markdown|enum" - to use without schema, otherwise it will be forced to json_object
}

/*
	##### PROVIDER OUTPUT #####
*/

// CodeExecution represents a server-side code execution result from the model.
// The model generates code, executes it in a sandboxed environment, and returns
// both the code and its execution result. The code and result are always paired:
// ExecutableCode contains the generated code, and the Outcome/Output fields
// contain the execution result.
//
// Currently supported by: Gemini (code_execution tool).
type CodeExecution struct {
	Language string `json:"language"`          // Programming language of the generated code (e.g., "PYTHON")
	Code     string `json:"code"`              // The code that was generated and executed
	Outcome  string `json:"outcome,omitempty"` // Execution outcome: "OUTCOME_OK", "OUTCOME_FAILED", "OUTCOME_DEADLINE_EXCEEDED"
	Output   string `json:"output,omitempty"`  // stdout on success, stderr or error description on failure
}

// Usage reports the token consumption for a single chat completion. Providers
// populate only the fields they support; unsupported counters remain zero.
// ReasoningTokens and CachedTokens are subset counts already included in
// PromptTokens / CompletionTokens; they are broken out for cost attribution.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`

	// Extended token metrics
	ReasoningTokens int `json:"reasoning_tokens,omitempty"` // Tokens used for reasoning (o1/o3/gpt-5)
	CachedTokens    int `json:"cached_tokens,omitempty"`    // Cached prompt tokens
}

// ChatResponse represents the completed response returned by a provider after a
// chat completion request. Content holds the primary text reply. Multimodal
// output (images, audio, video) is stored in the respective slice fields.
// ToolCalls is populated when the model requests one or more function calls
// instead of (or in addition to) generating text.
type ChatResponse struct {
	Id           string      `json:"id"`
	Model        string      `json:"model"`
	Object       string      `json:"object"`
	Created      int64       `json:"created"`
	Content      string      `json:"content"`
	Images       []ImageData `json:"images,omitempty"` // Generated images from the model response
	Audio        []AudioData `json:"audio,omitempty"`  // Generated audio from the model response (TTS, native audio)
	Videos       []VideoData `json:"videos,omitempty"` // Generated video from the model response
	ToolCalls    []ToolCall  `json:"tool_calls,omitempty"`
	FinishReason string      `json:"finish_reason,omitempty"`
	Usage        *Usage      `json:"usage,omitempty"`

	// Code execution results from server-side sandbox execution.
	// Currently supported by: Gemini (code_execution tool).
	// Each entry pairs the generated code with its execution outcome.
	CodeExecutions []CodeExecution `json:"code_executions,omitempty"`

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

	// URLContextSources contains metadata about URLs retrieved by the URL context tool.
	// Each entry describes a URL that the model fetched for grounding context.
	// Currently supported by: Gemini (url_context tool).
	URLContextSources []URLContextSource `json:"url_context_sources,omitempty"`
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

// URLContextSource represents a URL that was retrieved by the URL context tool
// for grounding the model's response. Contains metadata about the retrieval status
// and the amount of content extracted from each URL.
//
// Currently supported by: Gemini (url_context tool).
type URLContextSource struct {
	URL                    string `json:"url"`                                // The URL that was retrieved
	Status                 string `json:"status"`                             // Retrieval status (e.g., "SUCCESS", "FAILED")
	RetrievedContentLength int    `json:"retrieved_content_length,omitempty"` // Length of content retrieved from the URL
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

// ToolCall represents a single function invocation that the model has requested.
// ID uniquely identifies this call within the response and must be echoed back
// in the corresponding tool-role message so the provider can correlate results.
type ToolCall struct {
	ID       string           `json:"id,omitempty"` // Unique identifier for this tool call
	Type     string           `json:"type"`         // "function"
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction carries the name and serialized JSON arguments for a single
// function invocation requested by the model.
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

// MessageRole identifies the author of a message within a conversation.
// It is a string alias so that provider conversion layers can compare and
// switch on roles without casting.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"    // System instructions/configuration
	RoleUser      MessageRole = "user"      // End-user message
	RoleAssistant MessageRole = "assistant" // Model-generated reply or tool call request
	RoleTool      MessageRole = "tool"      // Tool/function output
)

/*
	##### MODEL METADATA #####
*/

// Modality represents an input or output modality supported by a model.
// Used in ModelInfo to describe what content types a model can accept and produce.
type Modality string

const (
	ModalityText     Modality = "text"     // Text input/output (universal)
	ModalityImage    Modality = "image"    // Image input (vision) or output (generation)
	ModalityAudio    Modality = "audio"    // Audio input (transcription) or output (TTS, native audio)
	ModalityVideo    Modality = "video"    // Video input (understanding) or output (generation)
	ModalityDocument Modality = "document" // Document input (PDF, plain text)
)

// ModelInfo describes a model's identity, capabilities, and pricing.
// Provider packages populate this from their model registries.
// This type is cross-provider compatible: Gemini, OpenAI, and Anthropic models
// can all be described with the same structure.
type ModelInfo struct {
	// ID is the canonical model identifier used in API calls (e.g., "gemini-2.5-pro", "gpt-4o").
	ID string `json:"id"`

	// Name is a human-readable display name (e.g., "Gemini 2.5 Pro").
	Name string `json:"name"`

	// Description is a short summary of the model's purpose and characteristics.
	Description string `json:"description,omitempty"`

	// InputModalities lists the content types the model can accept as input.
	InputModalities []Modality `json:"input_modalities"`

	// OutputModalities lists the content types the model can produce as output.
	OutputModalities []Modality `json:"output_modalities"`

	// Pricing holds the cost structure for this model. Nil if pricing is unavailable
	// (e.g., preview/experimental models with unpublished pricing).
	Pricing *cost.ModelCost `json:"pricing,omitempty"`

	// Deprecated indicates whether this model is deprecated and should be avoided.
	Deprecated bool `json:"deprecated,omitempty"`
}
