package gemini

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/leofalp/aigo/providers/ai"
)

// requestToGemini converts an ai.ChatRequest to a Gemini generateContentRequest.
func requestToGemini(request ai.ChatRequest) generateContentRequest {
	req := generateContentRequest{}

	// Build system instruction
	if request.SystemPrompt != "" {
		req.SystemInstruction = &systemInstruction{
			Parts: []part{{Text: request.SystemPrompt}},
		}
	}

	// Build contents from messages
	req.Contents = buildContents(request.Messages)

	// Build generation config
	req.GenerationConfig = buildGenerationConfig(request.GenerationConfig, request.ResponseFormat)

	// Build tools
	if len(request.Tools) > 0 {
		req.Tools = buildTools(request.Tools)
		req.ToolConfig = buildToolConfig(request.ToolChoice)
	}

	// Build safety settings
	if request.GenerationConfig != nil && len(request.GenerationConfig.SafetySettings) > 0 {
		req.SafetySettings = buildSafetySettings(request.GenerationConfig.SafetySettings)
	}

	return req
}

// buildContents converts ai.Message slice to Gemini content slice.
// Role mapping: user -> user, assistant -> model, tool -> user with functionResponse
func buildContents(messages []ai.Message) []content {
	var contents []content

	for _, msg := range messages {
		switch msg.Role {
		case ai.RoleUser:
			userContent := content{Role: "user"}
			// Use multimodal ContentParts if available, otherwise fall back to text Content
			if len(msg.ContentParts) > 0 {
				userContent.Parts = contentPartsToGeminiParts(msg.ContentParts)
			} else {
				userContent.Parts = []part{{Text: msg.Content}}
			}
			contents = append(contents, userContent)

		case ai.RoleAssistant:
			c := content{Role: "model"}

			// Handle tool calls
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					c.Parts = append(c.Parts, part{
						FunctionCall: &functionCall{
							Name: tc.Function.Name,
							Args: json.RawMessage(tc.Function.Arguments),
						},
					})
				}
			}

			// Use multimodal ContentParts if available, otherwise fall back to text Content
			if len(msg.ContentParts) > 0 {
				c.Parts = append(c.Parts, contentPartsToGeminiParts(msg.ContentParts)...)
			} else if msg.Content != "" {
				c.Parts = append(c.Parts, part{Text: msg.Content})
			}

			if len(c.Parts) > 0 {
				contents = append(contents, c)
			}

		case ai.RoleTool:
			// Tool responses in Gemini are sent as user role with functionResponse part
			contents = append(contents, content{
				Role: "user",
				Parts: []part{{
					FunctionResponse: &functionResponse{
						Name:     msg.Name,
						Response: json.RawMessage(msg.Content),
					},
				}},
			})

		case ai.RoleSystem:
			// System messages should go to SystemInstruction, not here
			// If someone passes a system message in Messages, convert to user message
			contents = append(contents, content{
				Role:  "user",
				Parts: []part{{Text: msg.Content}},
			})
		}
	}

	return contents
}

// contentPartsToGeminiParts converts generic ContentPart slices to Gemini part slices.
// For each content type, the conversion chooses between inlineData (base64) and fileData (URI)
// based on which field is populated. If both Data and URI are set, URI takes precedence.
func contentPartsToGeminiParts(contentParts []ai.ContentPart) []part {
	var parts []part
	for _, contentPart := range contentParts {
		switch contentPart.Type {
		case ai.ContentTypeText:
			parts = append(parts, part{Text: contentPart.Text})

		case ai.ContentTypeImage:
			if contentPart.Image != nil {
				parts = append(parts, mediaDataToPart(contentPart.Image.MimeType, contentPart.Image.Data, contentPart.Image.URI))
			}

		case ai.ContentTypeAudio:
			if contentPart.Audio != nil {
				parts = append(parts, mediaDataToPart(contentPart.Audio.MimeType, contentPart.Audio.Data, contentPart.Audio.URI))
			}

		case ai.ContentTypeVideo:
			if contentPart.Video != nil {
				parts = append(parts, mediaDataToPart(contentPart.Video.MimeType, contentPart.Video.Data, contentPart.Video.URI))
			}

		case ai.ContentTypeDocument:
			if contentPart.Document != nil {
				parts = append(parts, mediaDataToPart(contentPart.Document.MimeType, contentPart.Document.Data, contentPart.Document.URI))
			}
			// TODO: implement document/PDF content part extraction from response
		}
	}
	return parts
}

// mediaDataToPart converts media data (base64 or URI) to a Gemini part.
// URI takes precedence over inline data when both are provided.
func mediaDataToPart(mimeType, data, uri string) part {
	if uri != "" {
		return part{
			FileData: &fileData{
				MimeType: mimeType,
				FileURI:  uri,
			},
		}
	}
	return part{
		InlineData: &inlineData{
			MimeType: mimeType,
			Data:     data,
		},
	}
}

// buildGenerationConfig converts ai.GenerationConfig and ai.ResponseFormat to Gemini generationConfig.
func buildGenerationConfig(cfg *ai.GenerationConfig, respFmt *ai.ResponseFormat) *generationConfig {
	if cfg == nil && respFmt == nil {
		return nil
	}

	gc := &generationConfig{}

	if cfg != nil {
		// Standard generation parameters
		if cfg.Temperature > 0 {
			t := float64(cfg.Temperature)
			gc.Temperature = &t
		}

		if cfg.TopP > 0 {
			p := float64(cfg.TopP)
			gc.TopP = &p
		}

		if cfg.MaxOutputTokens > 0 {
			gc.MaxOutputTokens = &cfg.MaxOutputTokens
		} else if cfg.MaxTokens > 0 {
			gc.MaxOutputTokens = &cfg.MaxTokens
		}

		if cfg.FrequencyPenalty != 0 {
			fp := float64(cfg.FrequencyPenalty)
			gc.FrequencyPenalty = &fp
		}

		if cfg.PresencePenalty != 0 {
			pp := float64(cfg.PresencePenalty)
			gc.PresencePenalty = &pp
		}

		// Thinking config (Gemini-specific)
		if cfg.ThinkingBudget != nil || cfg.IncludeThoughts {
			gc.ThinkingConfig = &thinkingConfig{
				ThinkingBudget:  cfg.ThinkingBudget,
				IncludeThoughts: cfg.IncludeThoughts,
			}
		}

		// Response modalities (e.g., ["TEXT", "IMAGE"] for image generation)
		if len(cfg.ResponseModalities) > 0 {
			gc.ResponseModalities = cfg.ResponseModalities
		}
	}

	// Response format
	if respFmt != nil && respFmt.OutputSchema != nil {
		gc.ResponseMimeType = "application/json"
		schemaBytes, err := json.Marshal(respFmt.OutputSchema)
		if err == nil {
			gc.ResponseSchema = schemaBytes
		}
	}

	return gc
}

// buildTools converts ai.ToolDescription slice to Gemini tool slice.
// Handles both built-in tools (google_search, url_context, code_execution) and user-defined functions.
func buildTools(aiTools []ai.ToolDescription) []tool {
	var result []tool
	var funcDecls []functionDeclaration

	for _, t := range aiTools {
		switch t.Name {
		case ai.ToolGoogleSearch:
			result = append(result, tool{GoogleSearch: &googleSearchTool{}})

		case ai.ToolURLContext:
			result = append(result, tool{URLContext: &urlContextTool{}})

		case ai.ToolCodeExecution:
			result = append(result, tool{CodeExecution: &codeExecutionTool{}})

		default:
			// User-defined function
			fd := functionDeclaration{
				Name:        t.Name,
				Description: t.Description,
			}
			if t.Parameters != nil {
				paramBytes, err := json.Marshal(t.Parameters)
				if err == nil {
					fd.Parameters = paramBytes
				}
			}
			funcDecls = append(funcDecls, fd)
		}
	}

	// Add function declarations as a single tool if any
	if len(funcDecls) > 0 {
		result = append(result, tool{FunctionDeclarations: funcDecls})
	}

	return result
}

// buildToolConfig converts ai.ToolChoice to Gemini toolConfig.
func buildToolConfig(tc *ai.ToolChoice) *toolConfig {
	if tc == nil {
		return nil
	}

	config := &toolConfig{
		FunctionCallingConfig: &functionCallingConfig{},
	}

	// Map tool choice to Gemini's function calling modes
	if tc.ToolChoiceForced != "" {
		switch strings.ToLower(tc.ToolChoiceForced) {
		case "none":
			config.FunctionCallingConfig.Mode = "NONE"
		case "auto":
			config.FunctionCallingConfig.Mode = "AUTO"
		case "required":
			config.FunctionCallingConfig.Mode = "ANY"
		default:
			// Specific tool name - use ANY mode with allowed function names
			config.FunctionCallingConfig.Mode = "ANY"
			config.FunctionCallingConfig.AllowedFunctionNames = []string{tc.ToolChoiceForced}
		}
	} else if tc.AtLeastOneRequired {
		config.FunctionCallingConfig.Mode = "ANY"
	} else if len(tc.RequiredTools) > 0 {
		config.FunctionCallingConfig.Mode = "ANY"
		for _, t := range tc.RequiredTools {
			config.FunctionCallingConfig.AllowedFunctionNames = append(
				config.FunctionCallingConfig.AllowedFunctionNames,
				t.Name,
			)
		}
	}

	return config
}

// buildSafetySettings converts ai.SafetySetting slice to Gemini safetySetting slice.
func buildSafetySettings(settings []ai.SafetySetting) []safetySetting {
	result := make([]safetySetting, len(settings))
	for i, s := range settings {
		result[i] = safetySetting{
			Category:  s.Category,
			Threshold: s.Threshold,
		}
	}
	return result
}

// geminiToGeneric converts a Gemini generateContentResponse to ai.ChatResponse.
func geminiToGeneric(resp generateContentResponse) *ai.ChatResponse {
	result := &ai.ChatResponse{
		Id:      fmt.Sprintf("gemini-%d", time.Now().UnixNano()),
		Model:   resp.ModelVersion,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
	}

	// Handle empty response
	if len(resp.Candidates) == 0 {
		result.FinishReason = "error"
		if resp.PromptFeedback != nil && resp.PromptFeedback.BlockReason != "" {
			result.FinishReason = "content_filter"
			result.Refusal = resp.PromptFeedback.BlockReason
		}
		return result
	}

	candidate := resp.Candidates[0]

	// Map finish reason
	result.FinishReason = mapFinishReason(candidate.FinishReason)

	// Extract content and tool calls
	if candidate.Content != nil {
		var textParts []string
		var reasoningParts []string

		for _, p := range candidate.Content.Parts {
			if p.Text != "" {
				if p.Thought {
					// This is a reasoning/thinking part
					reasoningParts = append(reasoningParts, p.Text)
				} else {
					// This is actual content
					textParts = append(textParts, p.Text)
				}
			}

			if p.FunctionCall != nil {
				result.ToolCalls = append(result.ToolCalls, ai.ToolCall{
					ID:   fmt.Sprintf("call_%d", len(result.ToolCalls)),
					Type: "function",
					Function: ai.ToolCallFunction{
						Name:      p.FunctionCall.Name,
						Arguments: string(p.FunctionCall.Args),
					},
				})
			}

			// Extract inline media data from response (images, audio, video)
			if p.InlineData != nil {
				switch {
				case isAudioMimeType(p.InlineData.MimeType):
					result.Audio = append(result.Audio, ai.AudioData{
						MimeType: p.InlineData.MimeType,
						Data:     p.InlineData.Data,
					})
				case isVideoMimeType(p.InlineData.MimeType):
					result.Videos = append(result.Videos, ai.VideoData{
						MimeType: p.InlineData.MimeType,
						Data:     p.InlineData.Data,
					})
				default:
					result.Images = append(result.Images, ai.ImageData{
						MimeType: p.InlineData.MimeType,
						Data:     p.InlineData.Data,
					})
				}
			}

			// Extract file-referenced media from response
			if p.FileData != nil {
				switch {
				case isAudioMimeType(p.FileData.MimeType):
					result.Audio = append(result.Audio, ai.AudioData{
						MimeType: p.FileData.MimeType,
						URI:      p.FileData.FileURI,
					})
				case isVideoMimeType(p.FileData.MimeType):
					result.Videos = append(result.Videos, ai.VideoData{
						MimeType: p.FileData.MimeType,
						URI:      p.FileData.FileURI,
					})
				default:
					result.Images = append(result.Images, ai.ImageData{
						MimeType: p.FileData.MimeType,
						URI:      p.FileData.FileURI,
					})
				}
			}
		}

		result.Content = strings.Join(textParts, "\n")
		result.Reasoning = strings.Join(reasoningParts, "\n")
	}

	// Update finish reason if tool calls present
	if len(result.ToolCalls) > 0 && result.FinishReason == "stop" {
		result.FinishReason = "tool_calls"
	}

	// Map usage
	if resp.UsageMetadata != nil {
		result.Usage = &ai.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
			ReasoningTokens:  resp.UsageMetadata.ThoughtsTokenCount,
			CachedTokens:     resp.UsageMetadata.CachedContentTokenCount,
		}
	}

	// Map grounding metadata
	if candidate.GroundingMetadata != nil {
		result.Grounding = mapGroundingMetadata(candidate.GroundingMetadata)
	}

	return result
}

// mapFinishReason converts Gemini finish reason to ai.ChatResponse finish reason.
func mapFinishReason(geminiReason string) string {
	switch geminiReason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	case "OTHER":
		return "stop"
	default:
		return "stop"
	}
}

// mapGroundingMetadata converts Gemini grounding metadata to the generic format.
func mapGroundingMetadata(gm *groundingMetadata) *ai.GroundingMetadata {
	if gm == nil {
		return nil
	}

	result := &ai.GroundingMetadata{
		SearchQueries: gm.WebSearchQueries,
	}

	// Map grounding chunks to sources
	for i, chunk := range gm.GroundingChunks {
		if chunk.Web != nil {
			result.Sources = append(result.Sources, ai.GroundingSource{
				Index: i,
				URI:   chunk.Web.URI,
				Title: chunk.Web.Title,
			})
		}
	}

	// Map grounding supports to citations
	for _, support := range gm.GroundingSupports {
		citation := ai.Citation{
			SourceIndices: support.GroundingChunkIndices,
			Confidence:    support.ConfidenceScores,
		}
		if support.Segment != nil {
			citation.Text = support.Segment.Text
			citation.StartIndex = support.Segment.StartIndex
			citation.EndIndex = support.Segment.EndIndex
		}
		result.Citations = append(result.Citations, citation)
	}

	return result
}

// isAudioMimeType returns true if the given MIME type represents audio content.
func isAudioMimeType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "audio/")
}

// isVideoMimeType returns true if the given MIME type represents video content.
func isVideoMimeType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/")
}
