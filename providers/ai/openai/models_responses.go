package openai

import (
	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/ai"
)

/*
	RESPONSES API - INPUT
*/

// responseCreateRequest is the full request for the `/v1/responses` endpoint
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
	Text               *textConfig            `json:"text,omitempty"`
	Tools              []responseTool         `json:"tools,omitempty"`
	ToolChoice         interface{}            `json:"tool_choice,omitempty"` // "auto", "none", "required" or object
	ParallelToolCalls  *bool                  `json:"parallel_tool_calls,omitempty"`
	Background         *bool                  `json:"background,omitempty"`
	Store              *bool                  `json:"store,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	Truncation         string                 `json:"truncation,omitempty"` // "auto"
	Include            []string               `json:"include,omitempty"`    // e.g. ["reasoning.encrypted_content"]
}

// inputItem represents an item in the input array
type inputItem struct {
	Role    string      `json:"role"`    // developer, user, assistant
	Content interface{} `json:"content"` // string or []contentItem
}

// contentItem for multimodal content
type contentItem struct {
	Type     string `json:"type"` // input_text, input_image, output_text, etc.
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// reasoningConfig for reasoning-capable models (o1, o3, gpt-5)
type reasoningConfig struct {
	Effort  string `json:"effort,omitempty"`  // "minimal", "low", "medium", "high"
	Summary string `json:"summary,omitempty"` // "auto", "concise", "detailed"
}

// textConfig controls output formatting (GPT-5)
type textConfig struct {
	Verbosity string `json:"verbosity,omitempty"` // "low", "medium", "high"
	Format    *struct {
		Type string `json:"type"` // "text", "json_object"
	} `json:"format,omitempty"`
}

// responseTool definitions
type responseTool struct {
	// Common tools
	Type string `json:"type"` // "web_search", "file_search", "code_interpreter", "function", "custom", "mcp", "computer_use_preview"`

	// For file_search
	VectorStoreIDs []string `json:"vector_store_ids,omitempty"`

	// For function calling
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Parameters  jsonschema.Schema `json:"parameters,omitempty"`
	Strict      bool              `json:"strict,omitempty"`

	// For custom tools (GPT-5)
	Format *customFormat `json:"format,omitempty"`

	// For MCP (Model Context Protocol)
	ServerLabel     string            `json:"server_label,omitempty"`
	ServerURL       string            `json:"server_url,omitempty"`
	RequireApproval string            `json:"require_approval,omitempty"` // "always", "never"
	Headers         map[string]string `json:"headers,omitempty"`

	// For computer_use_preview
	DisplayWidth  int    `json:"display_width,omitempty"`
	DisplayHeight int    `json:"display_height,omitempty"`
	Environment   string `json:"environment,omitempty"` // "browser", "mac", "windows", "ubuntu"
}

type customFormat struct {
	Type       string `json:"type"`       // "grammar"
	Syntax     string `json:"syntax"`     // "lark"
	Definition string `json:"definition"` // Grammar definition
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
	Status             string                 `json:"status"` // "completed", "in_progress", "failed", "cancelled"
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

// outputItem represents an element in the `output` array
type outputItem struct {
	ID               string          `json:"id"`
	Type             string          `json:"type"`           // "message", "reasoning", "function_call", "web_search_call", etc.
	Role             string          `json:"role,omitempty"` // "assistant"
	Content          []contentOutput `json:"content,omitempty"`
	Status           string          `json:"status,omitempty"`
	Summary          []summaryItem   `json:"summary,omitempty"`
	EncryptedContent *string         `json:"encrypted_content,omitempty"`

	// For function calls
	Name      string `json:"name,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Arguments string `json:"arguments,omitempty"` // JSON string, parsed later with ParseStringAs
	Input     string `json:"input,omitempty"`     // For custom tools
}

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

// requestToResponses converts ai.ChatRequest into OpenAI Responses API request format
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
	for _, msg := range request.Messages {
		input = append(input, inputItem{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	// If there is a single user message without system prompt, use simple input
	var finalInput interface{}
	if len(input) == 1 && input[0].Role == "user" {
		finalInput = input[0].Content.(string)
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

		// Responses API prefers `max_output_tokens`
		if cfg.MaxOutputTokens > 0 {
			req.MaxOutputTokens = &cfg.MaxOutputTokens
		} else if cfg.MaxTokens > 0 {
			req.MaxOutputTokens = &cfg.MaxTokens
		}

		// Note: FrequencyPenalty and PresencePenalty are not supported by the Responses API
	}

	// Convert tools
	if len(request.Tools) > 0 {
		for _, tl := range request.Tools {
			req.Tools = append(req.Tools, responseTool{
				Type:        "function",
				Name:        tl.Name,
				Description: tl.Description,
				Parameters:  *tl.Parameters,
			})
		}

		// Set tool_choice with priority handling
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
					// Multiple required tools - Responses API supports arrays for forcing specific tools
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

	// Handle ResponseFormat
	if request.ResponseFormat != nil {
		// If a schema is provided, use structured output
		if request.ResponseFormat.OutputSchema != nil {
			req.Text = &textConfig{}
			req.Text.Format = &struct {
				Type string `json:"type"`
			}{
				Type: "json_object",
			}
		} else if request.ResponseFormat.Type != "" {
			// Use provided type hint
			formatType := request.ResponseFormat.Type
			if formatType == "json_schema" {
				formatType = "json_object"
			}
			req.Text = &textConfig{}
			req.Text.Format = &struct {
				Type string `json:"type"`
			}{
				Type: formatType,
			}
		}
	}

	return req
}

// responsesToGeneric converts OpenAI response into the generic ai.ChatResponse
func responsesToGeneric(resp responseCreateResponse) *ai.ChatResponse {
	chatResp := &ai.ChatResponse{
		Id:      resp.ID,
		Model:   resp.Model,
		Object:  resp.Object,
		Created: int64(resp.CreatedAt),
	}

	// Extract content and tool calls from output
	var contentParts []string
	var toolCalls []ai.ToolCall

	for _, output := range resp.Output {
		switch output.Type {
		case "message":
			// Extract text from message
			for _, content := range output.Content {
				if content.Type == "output_text" {
					contentParts = append(contentParts, content.Text)
				}
			}

		case "function_call":
			// Tool call from responses API
			// Convert Arguments from json.RawMessage to string
			// API already returns valid JSON, no need for ParseStringAs
			toolCalls = append(toolCalls, ai.ToolCall{
				Type: "function",
				Function: ai.ToolCallFunction{
					Name:      output.Name,
					Arguments: output.Arguments,
				},
			})

		case "reasoning":
			// Ignore or log reasoning items (no direct equivalent in ai.ChatResponse)
			continue

		case "web_search_call", "file_search_call", "code_interpreter_call":
			// Native tool calls - ignored for now
			continue
		}
	}

	// Combine all content parts
	if len(contentParts) > 0 {
		chatResp.Content = contentParts[0]
		if len(contentParts) > 1 {
			for i := 1; i < len(contentParts); i++ {
				chatResp.Content += "\n" + contentParts[i]
			}
		}
	}

	// Set tool calls
	if len(toolCalls) > 0 {
		chatResp.ToolCalls = toolCalls
	}

	// Map usage if present
	if resp.Usage != nil {
		chatResp.Usage = &ai.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	// Derive finish reason from status
	switch resp.Status {
	case "completed":
		if len(toolCalls) > 0 {
			chatResp.FinishReason = "tool_calls"
		} else {
			chatResp.FinishReason = "stop"
		}
	case "failed":
		chatResp.FinishReason = "error"
	case "cancelled":
		chatResp.FinishReason = "cancelled"
	default:
		chatResp.FinishReason = resp.Status
	}

	return chatResp
}

// parseResponsesToolCallsFromContent attempts to parse tool calls from message content using ParseStringAs.
// This is a fallback for edge cases and handles broken JSON gracefully.
func parseResponsesToolCallsFromContent(content string) []ai.ToolCall {
	var toolCalls []ai.ToolCall

	// Use ParseStringAs for robust JSON parsing with auto-repair
	functionCall, err := utils.ParseStringAs[ai.ToolCallFunction](content)
	if err == nil && functionCall.Name != "" {
		toolCalls = append(toolCalls, ai.ToolCall{
			Type:     "function",
			Function: functionCall,
		})
	}

	return toolCalls
}
