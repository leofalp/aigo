package openai

import (
	"aigo/internal/jsonschema"
	"aigo/providers/ai"
	"encoding/json"
)

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

type contentPart struct {
	Type     string           `json:"type"` // "text", "image_url"
	Text     string           `json:"text,omitempty"`
	ImageURL *contentImageURL `json:"image_url,omitempty"`
}

type contentImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
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
	ID       string               `json:"id"`
	Type     string               `json:"type"` // "function"
	Function chatToolCallFunction `json:"function"`
}

type chatToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type chatResponseFormat struct {
	Type       string                `json:"type"` // "text", "json_object", "json_schema"
	JSONSchema *chatJSONSchemaFormat `json:"json_schema,omitempty"`
}

type chatJSONSchemaFormat struct {
	Name   string            `json:"name"`
	Schema jsonschema.Schema `json:"schema"`
	Strict bool              `json:"strict,omitempty"`
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
	Refusal   string         `json:"refusal,omitempty"` // If model refuses
}

type chatUsage struct {
	PromptTokens            int                      `json:"prompt_tokens"`
	CompletionTokens        int                      `json:"completion_tokens"`
	TotalTokens             int                      `json:"total_tokens"`
	CompletionTokensDetails *chatTokensDetails       `json:"completion_tokens_details,omitempty"`
	PromptTokensDetails     *chatPromptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

type chatTokensDetails struct {
	ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty"`
}

type chatPromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
	AudioTokens  int `json:"audio_tokens,omitempty"`
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
			Role:    "system",
			Content: request.SystemPrompt,
		})
	}

	for _, msg := range request.Messages {
		req.Messages = append(req.Messages, chatMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
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
		if useLegacyFunctions {
			// Use legacy functions format
			for _, tl := range request.Tools {
				req.Functions = append(req.Functions, chatFunction{
					Name:        tl.Name,
					Description: tl.Description,
					Parameters:  *tl.Parameters,
				})
			}

			// Set function_call
			if request.ToolChoiceForced != "" {
				req.FunctionCall = request.ToolChoiceForced
			} else {
				hasRequired := false
				for _, tl := range request.Tools {
					if tl.Required {
						hasRequired = true
						break
					}
				}
				if hasRequired {
					req.FunctionCall = "auto" // TODO: support specific function forcing
				} else {
					req.FunctionCall = "auto"
				}
			}
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

			// Set tool_choice
			if request.ToolChoiceForced != "" {
				req.ToolChoice = request.ToolChoiceForced
			} else {
				hasRequired := false
				for _, tl := range request.Tools {
					if tl.Required {
						hasRequired = true
						break
					}
				}
				if hasRequired {
					req.ToolChoice = "required"
				} else {
					req.ToolChoice = "auto"
				}
			}
		}
	}

	// Handle ResponseFormat
	if request.ResponseFormat != nil {
		if request.ResponseFormat.OutputSchema != nil {
			// Structured output with schema
			req.ResponseFormat = &chatResponseFormat{
				Type: "json_schema",
				JSONSchema: &chatJSONSchemaFormat{
					Name:   "response_schema",
					Schema: *request.ResponseFormat.OutputSchema,
					Strict: request.ResponseFormat.Strict,
				},
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

	chatResp := &ai.ChatResponse{
		Id:           resp.ID,
		Model:        resp.Model,
		Object:       resp.Object,
		Created:      resp.Created,
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
	}

	// Convert tool calls
	if len(choice.Message.ToolCalls) > 0 {
		for _, tc := range choice.Message.ToolCalls {
			chatResp.ToolCalls = append(chatResp.ToolCalls, ai.ToolCall{
				Type: tc.Type,
				Function: ai.ToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	// Map usage
	if resp.Usage != nil {
		chatResp.Usage = &ai.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	return chatResp
}

// parseChatCompletionToolCallsFromContent attempts to parse tool calls from content
// This is a fallback for providers that don't properly format tool_calls
func parseChatCompletionToolCallsFromContent(content string) []ai.ToolCall {
	var toolCalls []ai.ToolCall

	// Try to parse as JSON array of tool calls
	var calls []struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal([]byte(content), &calls); err == nil {
		for _, call := range calls {
			toolCalls = append(toolCalls, ai.ToolCall{
				Type: "function",
				Function: ai.ToolCallFunction{
					Name:      call.Name,
					Arguments: string(call.Arguments),
				},
			})
		}
	}

	return toolCalls
}
