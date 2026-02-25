package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/leofalp/aigo/providers/ai"
)

// requestToAnthropic converts an ai.ChatRequest and provider Capabilities into
// an anthropicRequest ready to POST to Anthropic's Messages API.
// GenerationConfig fields are optional; safe defaults are applied when absent.
func requestToAnthropic(request ai.ChatRequest, capabilities Capabilities) (anthropicRequest, error) {
	req := anthropicRequest{
		Model:    request.Model,
		Messages: buildMessages(request.Messages),
	}

	// --- System prompt ---
	// Anthropic accepts the system field as either a plain JSON string or an
	// array of content blocks. Prompt caching requires the block array form so
	// that cache_control can be attached to the system content.
	if request.SystemPrompt != "" {
		if capabilities.PromptCaching {
			// Wrap in a content-block array so we can attach cache_control.
			systemBlocks := []anthropicContentBlock{
				{
					Type:         "text",
					Text:         request.SystemPrompt,
					CacheControl: &anthropicCacheControl{Type: "ephemeral"},
				},
			}
			systemBytes, err := json.Marshal(systemBlocks)
			if err != nil {
				return anthropicRequest{}, fmt.Errorf("failed to marshal system blocks: %w", err)
			}
			req.System = systemBytes
		} else {
			// Plain JSON string — simpler and slightly smaller on the wire.
			systemBytes, err := json.Marshal(request.SystemPrompt)
			if err != nil {
				return anthropicRequest{}, fmt.Errorf("failed to marshal system prompt: %w", err)
			}
			req.System = systemBytes
		}
	}

	// --- GenerationConfig ---
	maxTokens := 4096 // Anthropic requires max_tokens on every request
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

		// MaxOutputTokens takes precedence over the legacy MaxTokens field,
		// mirroring the priority used by the Gemini conversion layer.
		if cfg.MaxOutputTokens > 0 {
			maxTokens = cfg.MaxOutputTokens
		} else if cfg.MaxTokens > 0 {
			maxTokens = cfg.MaxTokens
		}

		// Extended thinking configuration.
		// IncludeThoughts or a non-nil ThinkingBudget both opt-in to thinking.
		// A ThinkingBudget of 0 explicitly disables thinking even when
		// IncludeThoughts is true.
		if cfg.IncludeThoughts || cfg.ThinkingBudget != nil {
			req.Thinking = buildThinkingConfig(cfg.ThinkingBudget)
		}
	}
	req.MaxTokens = maxTokens

	// --- Capabilities mapping ---
	if capabilities.Effort != "" {
		req.OutputConfig = &anthropicOutputConfig{Effort: capabilities.Effort}
	}
	if capabilities.Speed != "" {
		req.Speed = capabilities.Speed
	}

	// --- Tools ---
	if len(request.Tools) > 0 {
		req.Tools = buildAnthropicTools(request.Tools, capabilities.PromptCaching)
		req.ToolChoice = buildAnthropicToolChoice(request.ToolChoice)
	}

	return req, nil
}

// buildThinkingConfig constructs an anthropicThinkingConfig based on the
// optional budget pointer.
//
//   - nil or -1 → adaptive (let the model decide)
//   - positive value → enabled with a fixed token budget
//   - 0 → caller should not invoke this function (thinking disabled)
func buildThinkingConfig(budget *int) *anthropicThinkingConfig {
	if budget == nil || *budget == -1 {
		return &anthropicThinkingConfig{Type: "adaptive"}
	}
	if *budget == 0 {
		// Explicit zero means "disable thinking" — return nil so no field is set.
		return nil
	}
	return &anthropicThinkingConfig{
		Type:         "enabled",
		BudgetTokens: *budget,
	}
}

// buildMessages converts a slice of ai.Message into Anthropic message objects.
//
// Anthropic requires strictly alternating user/assistant turns. Consecutive
// tool-result messages (ai.RoleTool) are therefore merged into a single user
// message with multiple tool_result content blocks, which is the only layout
// the API accepts.
func buildMessages(messages []ai.Message) []anthropicMessage {
	var result []anthropicMessage

	for _, msg := range messages {
		switch msg.Role {
		case ai.RoleUser:
			userMsg := anthropicMessage{Role: "user"}
			if len(msg.ContentParts) > 0 {
				userMsg.Content = contentPartsToAnthropicBlocks(msg.ContentParts)
			} else {
				userMsg.Content = []anthropicContentBlock{
					{Type: "text", Text: msg.Content},
				}
			}
			result = append(result, userMsg)

		case ai.RoleAssistant:
			assistantMsg := anthropicMessage{Role: "assistant"}

			// Thinking blocks must come before any text or tool_use blocks so
			// that the API can verify the round-trip signature when caching.
			if msg.Reasoning != "" {
				assistantMsg.Content = append(assistantMsg.Content, anthropicContentBlock{
					Type:     "thinking",
					Thinking: msg.Reasoning,
				})
			}

			// Tool calls are represented as tool_use blocks.
			for _, toolCall := range msg.ToolCalls {
				assistantMsg.Content = append(assistantMsg.Content, anthropicContentBlock{
					Type:  "tool_use",
					ID:    toolCall.ID,
					Name:  toolCall.Function.Name,
					Input: json.RawMessage(toolCall.Function.Arguments),
				})
			}

			// Text content — prefer ContentParts for multimodal, fall back to Content.
			if len(msg.ContentParts) > 0 {
				assistantMsg.Content = append(assistantMsg.Content, contentPartsToAnthropicBlocks(msg.ContentParts)...)
			} else if msg.Content != "" {
				assistantMsg.Content = append(assistantMsg.Content, anthropicContentBlock{
					Type: "text",
					Text: msg.Content,
				})
			}

			if len(assistantMsg.Content) > 0 {
				result = append(result, assistantMsg)
			}

		case ai.RoleTool:
			// Marshal the tool result content as a JSON string so Anthropic
			// receives a well-formed JSON value in the content field.
			toolResultContent, err := json.Marshal(msg.Content)
			if err != nil {
				// Fallback to the raw string wrapped in quotes.
				toolResultContent = []byte(`"` + msg.Content + `"`)
			}

			toolResultBlock := anthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   toolResultContent,
			}

			// Merge consecutive tool results into a single user message.
			// Anthropic forbids two consecutive user turns, so multiple tool
			// responses must be combined into one message.
			if len(result) > 0 && isAllToolResults(result[len(result)-1]) {
				result[len(result)-1].Content = append(result[len(result)-1].Content, toolResultBlock)
			} else {
				result = append(result, anthropicMessage{
					Role:    "user",
					Content: []anthropicContentBlock{toolResultBlock},
				})
			}

		case ai.RoleSystem:
			// System messages belong in the top-level system field, not here.
			// Handle them defensively as user messages to avoid a silent drop.
			result = append(result, anthropicMessage{
				Role:    "user",
				Content: []anthropicContentBlock{{Type: "text", Text: msg.Content}},
			})
		}
	}

	return result
}

// isAllToolResults returns true when every content block in msg is a
// tool_result block. This is used to identify the last message as a
// mergeable tool-result turn so consecutive tool messages can be combined.
func isAllToolResults(msg anthropicMessage) bool {
	if msg.Role != "user" || len(msg.Content) == 0 {
		return false
	}
	for _, block := range msg.Content {
		if block.Type != "tool_result" {
			return false
		}
	}
	return true
}

// contentPartsToAnthropicBlocks converts generic ContentPart values into
// Anthropic content blocks. Unsupported content types (audio, video) are
// silently skipped because Anthropic does not currently support them.
func contentPartsToAnthropicBlocks(parts []ai.ContentPart) []anthropicContentBlock {
	var blocks []anthropicContentBlock

	for _, part := range parts {
		switch part.Type {
		case ai.ContentTypeText:
			blocks = append(blocks, anthropicContentBlock{
				Type: "text",
				Text: part.Text,
			})

		case ai.ContentTypeImage:
			if part.Image == nil {
				continue
			}
			block := anthropicContentBlock{Type: "image"}
			if part.Image.URI != "" {
				block.Source = &anthropicSource{
					Type: "url",
					URL:  part.Image.URI,
				}
			} else {
				block.Source = &anthropicSource{
					Type:      "base64",
					MediaType: part.Image.MimeType,
					Data:      part.Image.Data,
				}
			}
			blocks = append(blocks, block)

		case ai.ContentTypeDocument:
			if part.Document == nil {
				continue
			}
			blocks = append(blocks, anthropicContentBlock{
				Type: "document",
				Source: &anthropicSource{
					Type:      "base64",
					MediaType: part.Document.MimeType,
					Data:      part.Document.Data,
				},
			})

			// Audio and video are not supported by Anthropic's Messages API; skip silently.
		}
	}

	return blocks
}

// buildAnthropicTools converts the provider-agnostic ToolDescription slice to
// Anthropic tool definitions. Built-in pseudo-tools (prefixed with "_") are
// filtered out because Anthropic does not recognize them.
//
// When promptCaching is true, cache_control is attached to the last tool only.
// This is the recommended Anthropic pattern for caching long tool lists: mark
// the final entry so everything up to and including it is cached together.
func buildAnthropicTools(tools []ai.ToolDescription, promptCaching bool) []anthropicTool {
	var result []anthropicTool

	for _, tool := range tools {
		// Skip provider-specific built-in pseudo-tools.
		if ai.IsBuiltinTool(tool.Name) {
			continue
		}

		toolEntry := anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
		}

		if tool.Parameters != nil {
			schemaBytes, err := json.Marshal(tool.Parameters)
			if err == nil {
				toolEntry.InputSchema = schemaBytes
			}
		} else {
			// Anthropic requires input_schema; send an empty object schema when
			// no parameters are defined so the request remains valid.
			toolEntry.InputSchema = json.RawMessage(`{"type":"object","properties":{}}`)
		}

		result = append(result, toolEntry)
	}

	// Attach cache_control to the last real tool so the full list is cached.
	if promptCaching && len(result) > 0 {
		result[len(result)-1].CacheControl = &anthropicCacheControl{Type: "ephemeral"}
	}

	return result
}

// buildAnthropicToolChoice converts an ai.ToolChoice to its Anthropic wire
// representation. Returns nil when no explicit tool choice is specified,
// letting the API apply its default ("auto") behavior.
func buildAnthropicToolChoice(tc *ai.ToolChoice) *anthropicToolChoice {
	if tc == nil {
		return nil
	}

	if tc.ToolChoiceForced != "" {
		forcedName := tc.ToolChoiceForced
		// "auto" and "any" are Anthropic type literals, not tool names.
		switch strings.ToLower(forcedName) {
		case "auto":
			return &anthropicToolChoice{Type: "auto"}
		case "any", "required":
			return &anthropicToolChoice{Type: "any"}
		default:
			// Specific tool name — force the model to call exactly this tool.
			return &anthropicToolChoice{Type: "tool", Name: forcedName}
		}
	}

	if tc.AtLeastOneRequired {
		return &anthropicToolChoice{Type: "any"}
	}

	// No tool choice constraint specified; let the API default to "auto".
	return nil
}

// anthropicToGeneric converts an Anthropic Messages API response to the
// provider-agnostic ai.ChatResponse format.
//
// Multiple text blocks are joined with newlines into a single Content string.
// Multiple thinking blocks are similarly joined into Reasoning. Unknown block
// types are silently skipped for forward-compatibility with future Anthropic
// content types.
func anthropicToGeneric(response anthropicResponse) *ai.ChatResponse {
	result := &ai.ChatResponse{
		Id:      response.ID,
		Model:   response.Model,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
	}

	var textParts []string
	var reasoningParts []string

	for _, block := range response.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)

		case "thinking":
			reasoningParts = append(reasoningParts, block.Thinking)

		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, ai.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ai.ToolCallFunction{
					Name: block.Name,
					// Input is already a JSON object; convert to the string form
					// that ToolCallFunction.Arguments expects.
					Arguments: string(block.Input),
				},
			})

		default:
			// Unknown block types are silently ignored to remain compatible
			// with future Anthropic API additions.
		}
	}

	result.Content = strings.Join(textParts, "\n")
	result.Reasoning = strings.Join(reasoningParts, "\n")
	result.FinishReason = mapStopReason(response.StopReason)

	// Map usage counters. CacheCreationInputTokens and CacheReadInputTokens are
	// sub-counts of InputTokens but are surfaced via CachedTokens so that the
	// cost layer can apply the discounted cache-read rate.
	result.Usage = &ai.Usage{
		PromptTokens:     response.Usage.InputTokens,
		CompletionTokens: response.Usage.OutputTokens,
		TotalTokens:      response.Usage.InputTokens + response.Usage.OutputTokens,
		CachedTokens:     response.Usage.CacheCreationInputTokens + response.Usage.CacheReadInputTokens,
	}

	return result
}

// mapStopReason converts an Anthropic stop_reason value to the canonical
// finish_reason string used by ai.ChatResponse.
func mapStopReason(stopReason string) string {
	switch stopReason {
	case "end_turn":
		return "stop"
	case "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	default:
		return "stop"
	}
}

// responseIDOrFallback returns the response ID when present, or a generated
// fallback value. Exported for stream.go reuse where partial responses may
// carry an empty ID field in early chunks.
func responseIDOrFallback(id string) string {
	if id != "" {
		return id
	}
	return fmt.Sprintf("anthropic-%d", time.Now().UnixNano())
}
