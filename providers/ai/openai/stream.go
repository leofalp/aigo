package openai

import (
	"context"
	"fmt"
	"io"

	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/observability"
)

// StreamMessage implements ai.StreamProvider for the OpenAI chat completions endpoint.
// It sends a streaming request with stream=true and returns a ChatStream that yields
// incremental deltas as SSE events arrive from the API.
//
// Only the /v1/chat/completions endpoint is supported for streaming. The /v1/responses
// endpoint uses a different SSE event schema and may be added in a future release.
func (provider *OpenAIProvider) StreamMessage(ctx context.Context, request ai.ChatRequest) (*ai.ChatStream, error) {
	// Enrich span if present in context
	span := observability.SpanFromContext(ctx)
	observer := observability.ObserverFromContext(ctx)

	if span != nil {
		span.AddEvent(observability.EventLLMRequestStart)
		span.SetAttributes(
			observability.String(observability.AttrLLMProvider, "openai"),
			observability.String(observability.AttrLLMEndpoint, provider.baseURL),
			observability.String(observability.AttrLLMModel, request.Model),
			observability.Bool("llm.streaming", true),
		)
	}

	if observer != nil {
		observer.Trace(ctx, "OpenAI provider preparing streaming request",
			observability.String(observability.AttrLLMProvider, "openai"),
			observability.String(observability.AttrLLMEndpoint, provider.baseURL),
			observability.String(observability.AttrLLMModel, request.Model),
			observability.Int(observability.AttrRequestMessagesCount, len(request.Messages)),
			observability.Int(observability.AttrRequestToolsCount, len(request.Tools)),
		)
	}

	// Check API key
	if provider.apiKey == "" {
		return nil, fmt.Errorf("API key is not set")
	}

	// Always use chat completions for streaming (responses endpoint has different SSE schema)
	useLegacyFunctions := (provider.capabilities.ToolCallMode == ToolCallModeFunctions)
	chatRequest := requestToChatCompletion(request, useLegacyFunctions)

	// Enable streaming with usage reporting
	streamEnabled := true
	chatRequest.Stream = &streamEnabled
	chatRequest.StreamOptions = &streamOptions{IncludeUsage: true}

	// Send the streaming request — body is left open for SSE reading
	streamURL := provider.baseURL + chatCompletionsEndpoint
	httpResponse, err := utils.DoPostStream(ctx, provider.client, streamURL, provider.apiKey, chatRequest)
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

			// Parse the SSE payload into a streaming chunk
			chunk, parseErr := unmarshalStreamChunk(payload)
			if parseErr != nil {
				yield(ai.StreamEvent{}, fmt.Errorf("failed to parse streaming chunk: %w", parseErr))
				return
			}

			// Convert chunk to StreamEvents and yield them
			events := openaiChunkToStreamEvents(chunk)
			for _, event := range events {
				if !yield(event, nil) {
					return // Caller stopped iterating
				}
			}
		}
	}

	return ai.NewChatStream(iteratorFunc), nil
}

// openaiChunkToStreamEvents converts a single OpenAI streaming chunk into one or more StreamEvents.
// A single chunk can carry multiple types of data (content + tool calls + usage).
func openaiChunkToStreamEvents(chunk *chatCompletionStreamChunk) []ai.StreamEvent {
	var events []ai.StreamEvent

	// Handle usage metadata (present in the final chunk when stream_options.include_usage is true).
	// Usage chunk typically has empty choices, so process it before choices.
	if chunk.Usage != nil {
		usage := &ai.Usage{
			PromptTokens:     chunk.Usage.PromptTokens,
			CompletionTokens: chunk.Usage.CompletionTokens,
			TotalTokens:      chunk.Usage.TotalTokens,
		}
		if chunk.Usage.CompletionTokensDetails != nil {
			usage.ReasoningTokens = chunk.Usage.CompletionTokensDetails.ReasoningTokens
		}
		if chunk.Usage.PromptTokensDetails != nil {
			usage.CachedTokens = chunk.Usage.PromptTokensDetails.CachedTokens
		}
		events = append(events, ai.StreamEvent{
			Type:  ai.StreamEventUsage,
			Usage: usage,
		})
	}

	for _, choice := range chunk.Choices {
		delta := choice.Delta

		// Content delta
		if delta.Content != nil && *delta.Content != "" {
			events = append(events, ai.StreamEvent{
				Type:    ai.StreamEventContent,
				Content: *delta.Content,
			})
		}

		// Reasoning delta
		if delta.Reasoning != nil && *delta.Reasoning != "" {
			events = append(events, ai.StreamEvent{
				Type:      ai.StreamEventReasoning,
				Reasoning: *delta.Reasoning,
			})
		}

		// Tool call deltas
		for _, toolCallPart := range delta.ToolCalls {
			events = append(events, ai.StreamEvent{
				Type: ai.StreamEventToolCall,
				ToolCall: &ai.ToolCallDelta{
					Index:     toolCallPart.Index,
					ID:        toolCallPart.ID,
					Name:      toolCallPart.Function.Name,
					Arguments: toolCallPart.Function.Arguments,
				},
			})
		}

		// Finish reason (done signal)
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			events = append(events, ai.StreamEvent{
				Type:         ai.StreamEventDone,
				FinishReason: *choice.FinishReason,
			})
		}
	}

	return events
}
