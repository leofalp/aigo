package openai

import (
	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
)

/*
	RESPONSES API - INPUT
*/

// responseCreateRequest is the full request body for `/v1/responses`
type responseCreateRequest struct {
	Model              string                 `json:"model"`
	Models             []string               `json:"models,omitempty"` // for model fallback
	Input              interface{}            `json:"input"`            // string or []inputItem
	PreviousResponseID string                 `json:"previous_response_id,omitempty"`
	Temperature        *float64               `json:"temperature,omitempty"`
	TopP               *float64               `json:"top_p,omitempty"`
	MaxOutputTokens    *int                   `json:"max_output_tokens,omitempty"`
	Stream             *bool                  `json:"stream,omitempty"`
	Reasoning          *reasoningConfig       `json:"reasoning,omitempty"`
	Text               *textConfig            `json:"text,omitempty"`            // legacy/simple formatting control
	ResponseFormat     *responseFormat        `json:"response_format,omitempty"` // structured or hinted output format
	Tools              []responseTool         `json:"tools,omitempty"`
	ToolChoice         interface{}            `json:"tool_choice,omitempty"` // "auto", "none", "required" or object/array
	ParallelToolCalls  *bool                  `json:"parallel_tool_calls,omitempty"`
	Background         *bool                  `json:"background,omitempty"`
	Store              *bool                  `json:"store,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	Truncation         string                 `json:"truncation,omitempty"` // "auto"
	Include            []string               `json:"include,omitempty"`    // e.g. ["reasoning.encrypted_content"]
}

// inputItem represents a single message (developer/user/assistant) for Responses API
type inputItem struct {
	Role    string      `json:"role"`    // developer, user, assistant
	Content interface{} `json:"content"` // string or []inputContentPart
}

// inputContentPart represents a Responses API multimodal content part.
type inputContentPart struct {
	Type       string               `json:"type"`
	Text       string               `json:"text,omitempty"`
	ImageURL   string               `json:"image_url,omitempty"`
	InputAudio *responsesInputAudio `json:"input_audio,omitempty"`
}

// responsesInputAudio describes inline audio input for Responses API.
type responsesInputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

// reasoningConfig for reasoning-capable models (o1, o3, gpt-5)
type reasoningConfig struct {
	Effort  string `json:"effort,omitempty"`  // "minimal", "low", "medium", "high"
	Summary string `json:"summary,omitempty"` // "auto", "concise", "detailed"
}

// textConfig provides simple output formatting (legacy field)
// For structured JSON Schema we now use ResponseFormat instead.
type textConfig struct {
	Verbosity string `json:"verbosity,omitempty"` // "low", "medium", "high"
	Format    *struct {
		Type string `json:"type"` // "text", "json_object"
	} `json:"format,omitempty"`
}

// responseFormat defines structured or hinted output for Responses API.
// Mirrors chat completions response_format semantics.
type responseFormat struct {
	Type       string `json:"type"` // "json_schema", "json_object", "text", "markdown", "enum", etc.
	JsonSchema *struct {
		Name   string            `json:"name"`             // arbitrary schema name
		Schema jsonschema.Schema `json:"schema"`           // actual JSON Schema
		Strict bool              `json:"strict,omitempty"` // strict adherence if supported
	} `json:"json_schema,omitempty"`
}

// responseTool describes a tool/function available to the model
type responseTool struct {
	Type string `json:"type"` // "web_search", "file_search", "code_interpreter", "function", ...

	// File search / vector-store specifics
	VectorStoreIDs []string `json:"vector_store_ids,omitempty"`

	// Function calling
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Parameters  jsonschema.Schema `json:"parameters,omitempty"`
	Strict      bool              `json:"strict,omitempty"`

	// Custom grammar-based tools
	Format *customFormat `json:"format,omitempty"`

	// MCP (Model Context Protocol) fields
	ServerLabel     string            `json:"server_label,omitempty"`
	ServerURL       string            `json:"server_url,omitempty"`
	RequireApproval string            `json:"require_approval,omitempty"` // "always", "never"
	Headers         map[string]string `json:"headers,omitempty"`

	// Computer use preview fields
	DisplayWidth  int    `json:"display_width,omitempty"`
	DisplayHeight int    `json:"display_height,omitempty"`
	Environment   string `json:"environment,omitempty"` // "browser", "mac", "windows", "ubuntu"
}

type customFormat struct {
	Type       string `json:"type"`       // "grammar"
	Syntax     string `json:"syntax"`     // "lark"
	Definition string `json:"definition"` // grammar definition
}

/*
	RESPONSES API - OUTPUT
*/

type responseCreateResponse struct {
	ID                 string                 `json:"id"`
	Object             string                 `json:"object"` // "response"
	CreatedAt          float64                `json:"created_at"`
	Model              string                 `json:"model"`
	Output             []outputItem           `json:"output"`
	Status             string                 `json:"status"` //nolint:misspell // OpenAI API values: "completed", "in_progress", "failed", "cancelled", "queued" or "incomplete"
	Tools              []responseTool         `json:"tools"`
	Usage              *usageDetails          `json:"usage,omitempty"`
	Temperature        float64                `json:"temperature"`
	TopP               float64                `json:"top_p"`
	MaxOutputTokens    *int                   `json:"max_output_tokens,omitempty"`
	PreviousResponseID *string                `json:"previous_response_id,omitempty"`
	Background         bool                   `json:"background"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	Error              *errorDetails          `json:"error,omitempty"`
}

// outputItem: one element in response.Output
type outputItem struct {
	ID               string          `json:"id"`
	Type             string          `json:"type"`           // "message", "reasoning", "function_call", ...
	Role             string          `json:"role,omitempty"` // "assistant"
	Content          []contentOutput `json:"content,omitempty"`
	Status           string          `json:"status,omitempty"`
	Summary          []summaryItem   `json:"summary,omitempty"`
	EncryptedContent *string         `json:"encrypted_content,omitempty"`

	// Function/tool call specifics
	Name      string `json:"name,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Arguments string `json:"arguments,omitempty"` // JSON string
	Input     string `json:"input,omitempty"`     // custom tools
}

// contentOutput for message output items
type contentOutput struct {
	Type        string       `json:"type"` // "output_text", "output_image"
	Text        string       `json:"text,omitempty"`
	ImageURL    string       `json:"image_url,omitempty"`
	Annotations []annotation `json:"annotations,omitempty"`
	Logprobs    *logprobs    `json:"logprobs,omitempty"`
}

type annotation struct {
	Type  string `json:"type"` // "url_citation"
	Title string `json:"title,omitempty"`
	URL   string `json:"url,omitempty"`
	Index *int   `json:"index,omitempty"`
}

type summaryItem struct {
	Text string `json:"text,omitempty"`
	Type string `json:"type"` // "summary_text"
}

type usageDetails struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	TotalTokens         int `json:"total_tokens"`
	OutputTokensDetails *struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details,omitempty"`
}

type logprobs struct {
	Content []tokenLogprob `json:"content"`
}

type tokenLogprob struct {
	Token       string       `json:"token"`
	Logprob     float64      `json:"logprob"`
	Bytes       []int        `json:"bytes"`
	TopLogprobs []topLogprob `json:"top_logprobs"`
}

type topLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []int   `json:"bytes"`
}

type errorDetails struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

/*
	CONVERSION FUNCTIONS
*/

// requestToResponses converts ai.ChatRequest into the OpenAI Responses API format.
func requestToResponses(request ai.ChatRequest) responseCreateRequest {
	// Build input from messages
	var input []inputItem

	// Add system prompt as developer message if present
	if request.SystemPrompt != "" {
		input = append(input, inputItem{
			Role:    "developer",
			Content: request.SystemPrompt,
		})
	}

	// Convert messages
	// Video and document types are not supported by the Responses API.
	for _, msg := range request.Messages {
		item := inputItem{
			Role:    string(msg.Role),
			Content: msg.Content,
		}

		if len(msg.ContentParts) > 0 {
			parts := make([]inputContentPart, 0, len(msg.ContentParts))
			for _, part := range msg.ContentParts {
				switch part.Type {
				case ai.ContentTypeText:
					parts = append(parts, inputContentPart{Type: "input_text", Text: part.Text})
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
					parts = append(parts, inputContentPart{Type: "input_image", ImageURL: imageURL})
				case ai.ContentTypeAudio:
					if part.Audio == nil || part.Audio.Data == "" {
						continue
					}
					format := mimeTypeToAudioFormat(part.Audio.MimeType)
					parts = append(parts, inputContentPart{Type: "input_audio", InputAudio: &responsesInputAudio{Data: part.Audio.Data, Format: format}})
				case ai.ContentTypeVideo, ai.ContentTypeDocument:
					// Responses API does not support video/document inputs.
					continue
				}
			}
			if len(parts) > 0 {
				item.Content = parts
			}
		}

		input = append(input, item)
	}

	// Optimize single user prompt without system prompt
	var finalInput interface{}
	if len(input) == 1 && input[0].Role == "user" {
		if content, ok := input[0].Content.(string); ok {
			finalInput = content
		} else {
			finalInput = input
		}
	} else {
		finalInput = input
	}

	// Build base request
	req := responseCreateRequest{
		Model: request.Model,
		Input: finalInput,
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

		// Responses API prefers max_output_tokens (fallback from MaxTokens)
		if cfg.MaxOutputTokens > 0 {
			req.MaxOutputTokens = &cfg.MaxOutputTokens
		} else if cfg.MaxTokens > 0 {
			req.MaxOutputTokens = &cfg.MaxTokens
		}
		// FrequencyPenalty / PresencePenalty not supported here.
	}

	// Convert tools
	if len(request.Tools) > 0 {
		for _, tl := range request.Tools {
			req.Tools = append(req.Tools, responseTool{
				Type:        "function",
				Name:        tl.Name,
				Description: tl.Description,
				Parameters:  *tl.Parameters,
				// Strict (per tool) not mapped yet; could propagate if needed.
			})
		}

		// Tool choice logic mirroring chat behavior
		var toolChoice any = "auto"

		if request.ToolChoice != nil {
			if request.ToolChoice.ToolChoiceForced != "" {
				toolChoice = request.ToolChoice.ToolChoiceForced
			} else if request.ToolChoice.AtLeastOneRequired {
				toolChoice = "required"
			} else if len(request.ToolChoice.RequiredTools) > 0 {
				if len(request.ToolChoice.RequiredTools) == 1 {
					toolChoice = map[string]any{
						"type": "function",
						"name": request.ToolChoice.RequiredTools[0].Name,
					}
				} else {
					// Array of forced tools
					var toolChoiceArray []map[string]any
					for _, tl := range request.ToolChoice.RequiredTools {
						toolChoiceArray = append(toolChoiceArray, map[string]any{
							"type": "function",
							"name": tl.Name,
						})
					}
					toolChoice = toolChoiceArray
				}
			}
		}

		req.ToolChoice = toolChoice
	}

	// Handle ResponseFormat (STRUCTURED OUTPUT FIX)
	if request.ResponseFormat != nil {
		switch {
		case request.ResponseFormat.OutputSchema != nil:
			// Proper JSON Schema structured output
			req.ResponseFormat = &responseFormat{
				Type: "json_schema",
				JsonSchema: &struct {
					Name   string            `json:"name"`
					Schema jsonschema.Schema `json:"schema"`
					Strict bool              `json:"strict,omitempty"`
				}{
					Name:   "response_schema",
					Schema: *request.ResponseFormat.OutputSchema,
					Strict: request.ResponseFormat.Strict,
				},
			}

		case request.ResponseFormat.Type != "":
			// Simple type hint (no schema)
			t := request.ResponseFormat.Type
			// If user asked for json_schema but no schema provided, degrade gracefully
			if t == "json_schema" {
				t = "json_object"
			}
			req.ResponseFormat = &responseFormat{Type: t}

		default:
			// No type, leave nil
		}
	}

	// NOTE: We keep Text.Format untouched for legacy/simple formatting.
	// If ResponseFormat is set to json_schema, models will honor schema over simple format.

	return req
}

// responsesToGeneric converts OpenAI Responses API response into the generic ai.ChatResponse.
func responsesToGeneric(resp responseCreateResponse) *ai.ChatResponse {
	chatResp := &ai.ChatResponse{
		Id:      resp.ID,
		Model:   resp.Model,
		Object:  resp.Object,
		Created: int64(resp.CreatedAt),
	}

	// Extract content and tool calls
	var contentParts []string
	var toolCalls []ai.ToolCall

	for _, output := range resp.Output {
		switch output.Type {
		case "message":
			for _, content := range output.Content {
				if content.Type == "output_text" {
					contentParts = append(contentParts, content.Text)
				}
				// TODO: extract output_image from Responses API output.
				// When content.Type == "output_image", populate chatResp.Images with
				// ai.ImageData{MimeType: "image/png", URI: content.ImageURL} (or decode
				// base64 inline data if provided). This enables image generation models
				// (e.g., gpt-image-1, dall-e-3) to return images through the generic response.
			}
		case "function_call":
			toolCalls = append(toolCalls, ai.ToolCall{
				Type: "function",
				Function: ai.ToolCallFunction{
					Name:      output.Name,
					Arguments: output.Arguments,
				},
			})
		case "reasoning":
			// Currently ignored; could aggregate reasoning channel separately.
			continue
		case "web_search_call", "file_search_call", "code_interpreter_call":
			// Native calls ignored for now.
			continue
		}
	}

	// Combine content
	if len(contentParts) > 0 {
		chatResp.Content = contentParts[0]
		for i := 1; i < len(contentParts); i++ {
			chatResp.Content += "\n" + contentParts[i]
		}
	}

	if len(toolCalls) > 0 {
		chatResp.ToolCalls = toolCalls
	}

	// Usage mapping
	if resp.Usage != nil {
		chatResp.Usage = &ai.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	// Finish reason derivation
	switch resp.Status {
	case "completed":
		if len(toolCalls) > 0 {
			chatResp.FinishReason = "tool_calls"
		} else {
			chatResp.FinishReason = "stop"
		}
	case "failed":
		chatResp.FinishReason = "error"
	case "cancelled": //nolint:misspell // OpenAI API uses British spelling
		chatResp.FinishReason = "cancelled" //nolint:misspell // OpenAI API uses British spelling
	default:
		chatResp.FinishReason = resp.Status
	}

	return chatResp
}
