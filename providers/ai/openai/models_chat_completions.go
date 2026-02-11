package openai

import (
	"encoding/json"
	"strings"

	"github.com/leofalp/aigo/core/parse"
	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
)

// contentPart represents a chat completions multimodal content part.
type contentPart struct {
	Type       string            `json:"type"`
	Text       string            `json:"text,omitempty"`
	ImageURL   *contentPartImage `json:"image_url,omitempty"`
	InputAudio *contentPartAudio `json:"input_audio,omitempty"`
}

// contentPartImage describes image content for chat completions.
type contentPartImage struct {
	URL string `json:"url"`
}

// contentPartAudio describes audio content for chat completions.
type contentPartAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

// buildDataURL formats base64 data into a data URL for OpenAI image inputs.
func buildDataURL(mimeType, data string) string {
	if mimeType == "" || data == "" {
		return ""
	}
	return "data:" + mimeType + ";base64," + data
}

// mimeTypeToAudioFormat converts a MIME type into the expected OpenAI audio format.
// Defaults to "wav" when the format is unknown.
func mimeTypeToAudioFormat(mimeType string) string {
	if mimeType == "" {
		return "wav"
	}
	mimeType = strings.ToLower(mimeType)
	mimeType = strings.TrimSpace(mimeType)
	mimeType = strings.TrimPrefix(mimeType, "audio/")
	if mimeType == "" {
		return "wav"
	}
	return mimeType
}

/*
	CHAT COMPLETIONS API - INPUT
*/

// chatCompletionRequest represents the /v1/chat/completions request format
type chatCompletionRequest struct {
	Model               string        `json:"model"`
	Models              []string      `json:"models,omitempty"` // for model fallback // TODO: implement model fallback logic
	Messages            []chatMessage `json:"messages"`
	Temperature         *float64      `json:"temperature,omitempty"`
	TopP                *float64      `json:"top_p,omitempty"`
	MaxTokens           *int          `json:"max_tokens,omitempty"`            // Legacy, still accepted
	MaxCompletionTokens *int          `json:"max_completion_tokens,omitempty"` // Preferred
	FrequencyPenalty    *float64      `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float64      `json:"presence_penalty,omitempty"`
	Stop                interface{}   `json:"stop,omitempty"` // string or []string
	Stream              *bool         `json:"stream,omitempty"`
	Seed                *int          `json:"seed,omitempty"`
	User                string        `json:"user,omitempty"`

	// Tool calling - new format
	Tools             []chatTool  `json:"tools,omitempty"`
	ToolChoice        interface{} `json:"tool_choice,omitempty"` // "auto", "none", "required", or object
	ParallelToolCalls *bool       `json:"parallel_tool_calls,omitempty"`

	// Tool calling - legacy format (for backwards compatibility)
	Functions    []chatFunction `json:"functions,omitempty"`
	FunctionCall interface{}    `json:"function_call,omitempty"` // "auto", "none", or object

	// Response format
	ResponseFormat *chatResponseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role       string         `json:"role"`              // system, user, assistant, tool
	Content    interface{}    `json:"content,omitempty"` // string or []contentPart for multimodal
	Name       string         `json:"name,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"` // For role=tool
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`   // For role=assistant
}

type chatTool struct {
	Type     string       `json:"type"` // "function"
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Parameters  jsonschema.Schema `json:"parameters,omitempty"`
	Strict      bool              `json:"strict,omitempty"`
}

type chatToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string, parsed later with ParseStringAs
	} `json:"function"`
}

type chatResponseFormat struct {
	Type       string `json:"type"` // "text", "json_object", "json_schema"
	JSONSchema *struct {
		Name   string            `json:"name"`
		Schema jsonschema.Schema `json:"schema"`
		Strict bool              `json:"strict,omitempty"`
	} `json:"json_schema,omitempty"`
}

/*
	CHAT COMPLETIONS API - OUTPUT
*/

type chatCompletionResponse struct {
	ID                string       `json:"id"`
	Object            string       `json:"object"` // "chat.completion"
	Created           int64        `json:"created"`
	Model             string       `json:"model"`
	SystemFingerprint string       `json:"system_fingerprint,omitempty"`
	Choices           []chatChoice `json:"choices"`
	Usage             *chatUsage   `json:"usage,omitempty"`

	// Azure/OpenAI safety filters
	PromptFilterResults []chatFilterResult `json:"prompt_filter_results,omitempty"`
	ServiceTier         string             `json:"service_tier,omitempty"`
}

type chatChoice struct {
	Index                int                       `json:"index"`
	Message              chatResponseMessage       `json:"message"`
	FinishReason         string                    `json:"finish_reason"` // "stop", "length", "tool_calls", "content_filter"
	Logprobs             interface{}               `json:"logprobs,omitempty"`
	ContentFilterResults *chatContentFilterResults `json:"content_filter_results,omitempty"`
}

type chatResponseMessage struct {
	Role      string         `json:"role"` // "assistant"
	Content   string         `json:"content,omitempty"`
	ToolCalls []chatToolCall `json:"tool_calls,omitempty"`
	Refusal   string         `json:"refusal,omitempty"`   // If model refuses
	Reasoning string         `json:"reasoning,omitempty"` // If model refuses
	// TODO reasoning detail from openrouter
}

type chatUsage struct {
	PromptTokens            int `json:"prompt_tokens"`
	CompletionTokens        int `json:"completion_tokens"`
	TotalTokens             int `json:"total_tokens"`
	CompletionTokensDetails *struct {
		ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
		AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty"`
		RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty"`
	} `json:"completion_tokens_details,omitempty"`
	PromptTokensDetails *struct {
		CachedTokens int `json:"cached_tokens,omitempty"`
		AudioTokens  int `json:"audio_tokens,omitempty"`
	} `json:"prompt_tokens_details,omitempty"`
}

type chatContentFilterResults struct {
	Hate     chatFilterResult `json:"hate"`
	SelfHarm chatFilterResult `json:"self_harm"`
	Sexual   chatFilterResult `json:"sexual"`
	Violence chatFilterResult `json:"violence"`
}

type chatFilterResult struct {
	Filtered bool   `json:"filtered"`
	Severity string `json:"severity"`
}

/*
	CONVERSION FUNCTIONS
*/

// requestToChatCompletion converts ai.ChatRequest to chat completions format
func requestToChatCompletion(request ai.ChatRequest, useLegacyFunctions bool) chatCompletionRequest {
	req := chatCompletionRequest{
		Model: request.Model,
	}

	// Convert messages
	if request.SystemPrompt != "" {
		req.Messages = append(req.Messages, chatMessage{
			Role:    string(ai.RoleSystem),
			Content: request.SystemPrompt,
		})
	}

	for _, msg := range request.Messages {
		chatMsg := chatMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}

		// Handle multimodal ContentParts for Chat Completions API.
		// Video and document types are not supported by the Chat Completions API.
		if len(msg.ContentParts) > 0 {
			parts := make([]contentPart, 0, len(msg.ContentParts))
			for _, part := range msg.ContentParts {
				switch part.Type {
				case ai.ContentTypeText:
					parts = append(parts, contentPart{Type: "text", Text: part.Text})
				case ai.ContentTypeImage:
					if part.Image == nil {
						continue
					}
					imageURL := part.Image.URI
					if imageURL == "" {
						imageURL = buildDataURL(part.Image.MimeType, part.Image.Data)
					}
					if imageURL == "" {
						continue
					}
					parts = append(parts, contentPart{Type: "image_url", ImageURL: &contentPartImage{URL: imageURL}})
				case ai.ContentTypeAudio:
					if part.Audio == nil || part.Audio.Data == "" {
						continue
					}
					format := mimeTypeToAudioFormat(part.Audio.MimeType)
					parts = append(parts, contentPart{Type: "input_audio", InputAudio: &contentPartAudio{Data: part.Audio.Data, Format: format}})
				case ai.ContentTypeVideo, ai.ContentTypeDocument:
					// Chat Completions API does not support video/document inputs.
					continue
				}
			}
			if len(parts) > 0 {
				chatMsg.Content = parts
			}
		}

		// Map tool-related fields
		if len(msg.ToolCalls) > 0 {
			// Convert ai.ToolCall to chatToolCall
			for _, tc := range msg.ToolCalls {
				toolCall := chatToolCall{
					ID:   tc.ID,
					Type: tc.Type,
				}
				toolCall.Function.Name = tc.Function.Name
				toolCall.Function.Arguments = tc.Function.Arguments
				chatMsg.ToolCalls = append(chatMsg.ToolCalls, toolCall)
			}
		}

		// Map tool response fields
		if msg.ToolCallID != "" {
			chatMsg.ToolCallID = msg.ToolCallID
		}
		if msg.Name != "" {
			chatMsg.Name = msg.Name
		}

		req.Messages = append(req.Messages, chatMsg)
	}

	// Map GenerationConfig
	if request.GenerationConfig != nil {
		cfg := request.GenerationConfig

		if cfg.Temperature > 0 {
			temp := float64(cfg.Temperature)
			req.Temperature = &temp
		}

		if cfg.TopP > 0 {
			topP := float64(cfg.TopP)
			req.TopP = &topP
		}

		if cfg.FrequencyPenalty != 0 {
			penalty := float64(cfg.FrequencyPenalty)
			req.FrequencyPenalty = &penalty
		}

		if cfg.PresencePenalty != 0 {
			penalty := float64(cfg.PresencePenalty)
			req.PresencePenalty = &penalty
		}

		// Prefer max_completion_tokens over max_tokens
		if cfg.MaxOutputTokens > 0 {
			req.MaxCompletionTokens = &cfg.MaxOutputTokens
		} else if cfg.MaxTokens > 0 {
			req.MaxTokens = &cfg.MaxTokens
		}
	}

	// Convert tools
	if len(request.Tools) > 0 {
		var toolChoice any = "auto" // Default to "auto" if not specified

		if request.ToolChoice != nil {
			// Priority 1: Explicit forced choice (e.g., "none", "auto", "required", or specific tool name)
			if request.ToolChoice.ToolChoiceForced != "" {
				toolChoice = request.ToolChoice.ToolChoiceForced
			} else if request.ToolChoice.AtLeastOneRequired {
				// Priority 2: Force at least one tool call
				toolChoice = "required"
			} else if len(request.ToolChoice.RequiredTools) > 0 {
				// Priority 3: Force specific tool(s)
				if len(request.ToolChoice.RequiredTools) == 1 {
					// Single required tool - force it specifically
					toolChoice = map[string]any{
						"type": "function",
						"name": request.ToolChoice.RequiredTools[0].Name,
					}
				} else {
					// Multiple required tools - use allowed_tools restriction (new format only)
					if useLegacyFunctions {
						// Legacy format doesn't support multiple forced tools, fallback to "required"
						toolChoice = "required"
					} else {
						toolChoice = map[string]any{
							"type": "allowed_tools",
							"mode": "required",
							"tools": func() []map[string]string {
								var tools []map[string]string
								for _, tl := range request.ToolChoice.RequiredTools {
									tools = append(tools, map[string]string{
										"type": "function",
										"name": tl.Name,
									})
								}
								return tools
							}(),
						}
					}
				}
			}
		}

		if useLegacyFunctions {
			// Use legacy functions format
			for _, tl := range request.Tools {
				req.Functions = append(req.Functions, chatFunction{
					Name:        tl.Name,
					Description: tl.Description,
					Parameters:  *tl.Parameters,
				})
			}
			req.FunctionCall = toolChoice
		} else {
			// Use new tools format
			for _, tl := range request.Tools {
				req.Tools = append(req.Tools, chatTool{
					Type: "function",
					Function: chatFunction{
						Name:        tl.Name,
						Description: tl.Description,
						Parameters:  *tl.Parameters,
					},
				})
			}
			req.ToolChoice = toolChoice
		}
	}

	// Handle ResponseFormat
	if request.ResponseFormat != nil {
		if request.ResponseFormat.OutputSchema != nil {
			// Structured output with schema
			req.ResponseFormat = &chatResponseFormat{
				Type: "json_schema",
			}
			req.ResponseFormat.JSONSchema = &struct {
				Name   string            `json:"name"`
				Schema jsonschema.Schema `json:"schema"`
				Strict bool              `json:"strict,omitempty"`
			}{
				Name:   "response_schema",
				Schema: *request.ResponseFormat.OutputSchema,
				Strict: request.ResponseFormat.Strict,
			}
		} else if request.ResponseFormat.Type != "" {
			// Simple type hint
			req.ResponseFormat = &chatResponseFormat{
				Type: request.ResponseFormat.Type,
			}
		}
	}

	return req
}

// chatCompletionToGeneric converts chat completion response to ai.ChatResponse
func chatCompletionToGeneric(resp chatCompletionResponse) *ai.ChatResponse {
	if len(resp.Choices) == 0 {
		return &ai.ChatResponse{
			Id:           resp.ID,
			Model:        resp.Model,
			Object:       resp.Object,
			Created:      resp.Created,
			FinishReason: "error",
		}
	}

	choice := resp.Choices[0]

	// reasoning could be into <think> tags in content
	// Extract reasoning from <think> tags if present
	content := strings.TrimSpace(choice.Message.Content)
	var reasoning string

	if content != "" {
		// if content is not empty, extract reasoning from both explicit field and <think> tags
		explicitReasoning := strings.TrimSpace(choice.Message.Reasoning)
		reasoning = explicitReasoning

		// handle in-content reasoning
		inContentReasoning := extractReasoningFromThinkTags(content)
		if inContentReasoning != "" {
			reasoning += "\n"
			reasoning += inContentReasoning
			content = cleanThinkTags(content) // remove reasoning (<think> tags) from content
		}
	} else if choice.Message.Reasoning != "" {
		// if content is empty, use reasoning field (may contain content)
		reasoning = extractReasoningFromThinkTags(choice.Message.Reasoning)
		content = cleanThinkTags(choice.Message.Reasoning)
	}

	chatResp := &ai.ChatResponse{
		Id:           resp.ID,
		Model:        resp.Model,
		Object:       resp.Object,
		Created:      resp.Created,
		Content:      content,
		Refusal:      choice.Message.Refusal,
		Reasoning:    reasoning,
		FinishReason: choice.FinishReason,
	}

	// Convert tool calls from standard format
	// Map tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		for _, tc := range choice.Message.ToolCalls {
			// Convert Arguments from json.RawMessage to string
			// API already returns valid JSON, no need for ParseStringAs
			chatResp.ToolCalls = append(chatResp.ToolCalls, ai.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: ai.ToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: string(tc.Function.Arguments),
				},
			})
		}
	} else if choice.Message.Content != "" {
		// Fallback: Try to parse tool calls from content
		// Some providers (e.g., OpenRouter with certain models) put tool calls in content
		parsed := parseChatCompletionToolCallsFromContent(choice.Message.Content)
		if len(parsed) > 0 {
			chatResp.ToolCalls = parsed
			// Update finish reason to indicate tool calls
			if chatResp.FinishReason == "stop" {
				chatResp.FinishReason = "tool_calls"
			}
		}
	}

	// Map usage
	if resp.Usage != nil {
		usage := &ai.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}

		// Map extended token metrics
		if resp.Usage.CompletionTokensDetails != nil {
			usage.ReasoningTokens = resp.Usage.CompletionTokensDetails.ReasoningTokens
		}
		if resp.Usage.PromptTokensDetails != nil {
			usage.CachedTokens = resp.Usage.PromptTokensDetails.CachedTokens
		}

		chatResp.Usage = usage
	}

	return chatResp
}

// parseChatCompletionToolCallsFromContent attempts to parse tool calls from content.
// This is a fallback for providers that don't properly format tool_calls in the standard field.
// It handles multiple formats:
// 1. <TOOLCALL>[{...}]</TOOLCALL> (some OpenRouter models)
// 2. Plain JSON array [{...}]
// 3. Provider-specific markers like <|END OF THOUGHT|>
// Note: JSON repair and escape sequences are handled by ParseStringAs.
func parseChatCompletionToolCallsFromContent(content string) []ai.ToolCall {
	var toolCalls []ai.ToolCall

	// Clean the content first
	cleaned := cleanToolCallContent(content)

	// Strategy 1: Extract content between <TOOLCALL> tags
	if strings.Contains(cleaned, "<TOOLCALL>") && strings.Contains(cleaned, "</TOOLCALL>") {
		start := strings.Index(cleaned, "<TOOLCALL>")
		end := strings.Index(cleaned, "</TOOLCALL>")
		if start != -1 && end > start {
			jsonContent := cleaned[start+10 : end] // +10 to skip "<TOOLCALL>"
			toolCalls = parseToolCallsJSON(jsonContent)
			if len(toolCalls) > 0 {
				return toolCalls
			}
		}
	}

	// Strategy 2: Try to parse entire content as JSON array
	toolCalls = parseToolCallsJSON(cleaned)
	if len(toolCalls) > 0 {
		return toolCalls
	}

	// Strategy 3: Try to find JSON array within content
	start := strings.Index(cleaned, "[")
	end := strings.LastIndex(cleaned, "]")
	if start != -1 && end > start {
		jsonContent := cleaned[start : end+1]
		toolCalls = parseToolCallsJSON(jsonContent)
		if len(toolCalls) > 0 {
			return toolCalls
		}
	}

	// Log only if we truly failed to parse anything
	// Note: Logging disabled to avoid dependencies, but we could add it if needed
	// if len(toolCalls) == 0 && len(cleaned) > 0 && (strings.Contains(cleaned, "TOOLCALL") || strings.Contains(cleaned, "name")) {
	//     slog.Debug("Could not parse tool calls from content", "content_preview", truncateForLog(cleaned, 100))
	// }

	return toolCalls
}

// cleanToolCallContent removes provider-specific markers that ParseStringAs doesn't handle.
// Note: Escape sequences and JSON repairs are now handled by ParseStringAs/jsonrepair.
func cleanToolCallContent(content string) string {
	content = strings.TrimSpace(content)

	// Remove common markers that providers add (not handled by jsonrepair)
	markers := []string{
		"<|END OF THOUGHT|>",
		"<|END_OF_THOUGHT|>",
		"<|endofthought|>",
		"[/TOOLCALL]",
		"</THOUGHT>",
		"<THOUGHT>",
	}
	for _, marker := range markers {
		content = strings.ReplaceAll(content, marker, "")
	}

	return strings.TrimSpace(content)
}

// parseToolCallsJSON attempts to parse a JSON string into tool calls using ParseStringAs for robustness.
// ParseStringAs handles JSON repair, escape sequences, comments, and other malformations automatically.
// We do minimal preprocessing for array brackets and trailing junk to help ParseStringAs.
func parseToolCallsJSON(jsonStr string) []ai.ToolCall {
	var toolCalls []ai.ToolCall

	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		return toolCalls
	}

	// Minimal preprocessing: ensure array brackets are present
	// This helps ParseStringAs when brackets are completely missing
	if !strings.HasPrefix(jsonStr, "[") {
		jsonStr = "[" + jsonStr
	}
	if !strings.HasSuffix(jsonStr, "]") {
		// Try to find the last complete object and add closing bracket
		lastBrace := strings.LastIndex(jsonStr, "}")
		if lastBrace > 0 {
			jsonStr = jsonStr[:lastBrace+1] + "]"
		} else {
			return toolCalls
		}
	}

	// Define the structure for parsing
	type toolCallParsed struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	// Use ParseStringAs - it handles JSON repair, trailing commas, escape sequences, etc.
	calls, err := parse.ParseStringAs[[]toolCallParsed](jsonStr)
	if err != nil {
		// ParseStringAs already tries to repair, so if it fails, we can't do much more
		return toolCalls
	}

	// Convert parsed calls to ai.ToolCall format
	for _, call := range calls {
		// Validate that we have at least a name
		if call.Name == "" {
			continue
		}

		// Convert arguments to JSON string
		var argsStr string
		if len(call.Arguments) > 0 {
			argsStr = string(call.Arguments)
		} else {
			argsStr = "{}"
		}

		toolCalls = append(toolCalls, ai.ToolCall{
			ID:   "", // Parsed tool calls from content don't have IDs
			Type: "function",
			Function: ai.ToolCallFunction{
				Name:      call.Name,
				Arguments: argsStr,
			},
		})
	}

	return toolCalls
}

// extractReasoningFromThinkTags extracts reasoning content from <think>...</think> tags.
// Some models (like DeepSeek) use these tags to show chain-of-thought reasoning.
// Returns the extracted reasoning text, or empty string if no tags found.
func extractReasoningFromThinkTags(content string) string {
	startTag := "<think>"
	endTag := "</think>"

	start := strings.Index(content, startTag)
	if start == -1 {
		start = 0 // if there's no start tag, consider from beginning
	} else {
		start += len(startTag)
	}

	end := strings.Index(content, endTag)
	if end == -1 || end <= start {
		return "" // mandatory end tag
	}

	// Extract content between tags
	reasoning := content[start:end]
	return strings.TrimSpace(reasoning)
}

// cleanThinkTags removes <think>...</think> tags and their content from the text.
// This leaves only the final answer/response without the reasoning part.
func cleanThinkTags(content string) string {
	startTag := "<think>"
	endTag := "</think>"

	start := strings.Index(content, startTag)
	if start == -1 {
		start = 0 // if there's no start tag, consider from beginning
	}

	end := strings.Index(content, endTag)
	if end == -1 || end <= start {
		return content // mantatory end tag
	}

	// Remove everything from start tag to end tag (inclusive)
	cleaned := content[:start] + content[end+len(endTag):]
	return strings.TrimSpace(cleaned)
}
