package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/observability"
)

// StreamMessage implements ai.StreamProvider for the Gemini API.
// It uses the streamGenerateContent endpoint with alt=sse to receive
// incremental response chunks as SSE events.
//
// Like OpenAI and Anthropic, Gemini streaming returns text deltas (only the new
// portion) in each SSE chunk, so events are passed through directly.
func (provider *GeminiProvider) StreamMessage(ctx context.Context, request ai.ChatRequest) (*ai.ChatStream, error) {
	// Get observability context
	span := observability.SpanFromContext(ctx)
	observer := observability.ObserverFromContext(ctx)

	// Determine model
	model := request.Model
	if model == "" {
		model = provider.defaultModel
	}

	if span != nil {
		span.AddEvent(observability.EventLLMRequestStart)
		span.SetAttributes(
			observability.String(observability.AttrLLMProvider, "gemini"),
			observability.String(observability.AttrLLMEndpoint, provider.baseURL),
			observability.String(observability.AttrLLMModel, model),
			observability.Bool("llm.streaming", true),
		)
	}

	if observer != nil {
		observer.Trace(ctx, "Gemini provider preparing streaming request",
			observability.String(observability.AttrLLMProvider, "gemini"),
			observability.String(observability.AttrLLMEndpoint, provider.baseURL),
			observability.String(observability.AttrLLMModel, model),
			observability.Int(observability.AttrRequestMessagesCount, len(request.Messages)),
			observability.Int(observability.AttrRequestToolsCount, len(request.Tools)),
		)
	}

	// Validate API key
	if provider.apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not set")
	}

	// Build streaming URL: streamGenerateContent with alt=sse
	streamURL := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", provider.baseURL, model)

	// Convert request to Gemini format (same as non-streaming)
	geminiRequest := requestToGemini(request)

	// Send the streaming request with Gemini-specific auth header
	httpResponse, err := utils.DoPostStream(
		ctx,
		provider.client,
		streamURL,
		"", // Empty apiKey for DoPostStream's default Bearer auth
		geminiRequest,
		utils.HeaderOption{Key: "x-goog-api-key", Value: provider.apiKey},
	)
	if err != nil {
		if observer != nil {
			observer.Trace(ctx, "Streaming HTTP request failed", observability.Error(err))
		}
		return nil, err
	}

	// Build the iterator function that reads SSE events and converts them to StreamEvents
	sseScanner := utils.NewSSEScanner(httpResponse.Body)

	iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
		// Ensure the response body is closed when the iterator is done
		defer utils.CloseWithLog(httpResponse.Body)

		toolCallsEmitted := false

		for {
			// Check for context cancellation
			if ctx.Err() != nil {
				yield(ai.StreamEvent{}, ctx.Err())
				return
			}

			payload, sseErr := sseScanner.Next()
			if sseErr == io.EOF {
				// Stream finished normally
				return
			}
			if sseErr != nil {
				yield(ai.StreamEvent{}, fmt.Errorf("SSE read error: %w", sseErr))
				return
			}

			// Each SSE event is a full generateContentResponse
			var geminiResponse generateContentResponse
			if parseErr := json.Unmarshal([]byte(payload), &geminiResponse); parseErr != nil {
				yield(ai.StreamEvent{}, fmt.Errorf("failed to parse Gemini streaming chunk: %w", parseErr))
				return
			}

			// Extract events from this chunk
			events := geminiChunkToStreamEvents(&geminiResponse, &toolCallsEmitted)
			for _, event := range events {
				if !yield(event, nil) {
					return // Caller stopped iterating
				}
			}
		}
	}

	return ai.NewChatStream(iteratorFunc), nil
}

// geminiChunkToStreamEvents converts a Gemini generateContentResponse (from streaming)
// into StreamEvents. Each SSE chunk carries text deltas (only the new portion),
// so text and reasoning parts are emitted directly without cumulative tracking.
// Tool calls are emitted as complete events (Gemini sends them whole, not incremental).
func geminiChunkToStreamEvents(
	response *generateContentResponse,
	toolCallsEmitted *bool,
) []ai.StreamEvent {
	var events []ai.StreamEvent

	if len(response.Candidates) == 0 {
		return events
	}

	firstCandidate := response.Candidates[0]
	if firstCandidate.Content == nil {
		// Check for finish reason even without content
		if firstCandidate.FinishReason != "" {
			events = append(events, ai.StreamEvent{
				Type:         ai.StreamEventDone,
				FinishReason: mapFinishReason(firstCandidate.FinishReason),
			})
		}
		return events
	}

	// Accumulate text and reasoning from all parts in this chunk
	var textParts []string
	var reasoningParts []string
	toolCallIndex := 0

	for _, part := range firstCandidate.Content.Parts {
		if part.Text != "" {
			if part.Thought {
				reasoningParts = append(reasoningParts, part.Text)
			} else {
				textParts = append(textParts, part.Text)
			}
		}

		// Tool calls are sent as complete function calls (not incremental like OpenAI).
		// Emit each one as a StreamEventToolCall. Only emit once (they appear in the final chunk).
		if part.FunctionCall != nil && !*toolCallsEmitted {
			events = append(events, ai.StreamEvent{
				Type: ai.StreamEventToolCall,
				ToolCall: &ai.ToolCallDelta{
					Index:     toolCallIndex,
					ID:        fmt.Sprintf("call_%d", toolCallIndex),
					Name:      part.FunctionCall.Name,
					Arguments: string(part.FunctionCall.Args),
				},
			})
			toolCallIndex++
		}
	}

	if toolCallIndex > 0 {
		*toolCallsEmitted = true
	}

	// Emit text delta directly — each Gemini streaming chunk already contains only the new text.
	for _, textPart := range textParts {
		events = append(events, ai.StreamEvent{
			Type:    ai.StreamEventContent,
			Content: textPart,
		})
	}

	// Emit reasoning delta directly — same delta-based behavior as text.
	for _, reasoningPart := range reasoningParts {
		events = append(events, ai.StreamEvent{
			Type:      ai.StreamEventReasoning,
			Reasoning: reasoningPart,
		})
	}

	// Usage metadata (typically in the final chunk)
	if response.UsageMetadata != nil {
		events = append(events, ai.StreamEvent{
			Type: ai.StreamEventUsage,
			Usage: &ai.Usage{
				PromptTokens:     response.UsageMetadata.PromptTokenCount,
				CompletionTokens: response.UsageMetadata.CandidatesTokenCount,
				TotalTokens:      response.UsageMetadata.TotalTokenCount,
				ReasoningTokens:  response.UsageMetadata.ThoughtsTokenCount,
				CachedTokens:     response.UsageMetadata.CachedContentTokenCount,
			},
		})
	}

	// Finish reason
	if firstCandidate.FinishReason != "" {
		events = append(events, ai.StreamEvent{
			Type:         ai.StreamEventDone,
			FinishReason: mapFinishReason(firstCandidate.FinishReason),
		})
	}

	return events
}
