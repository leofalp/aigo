package anthropic

import (
	"context"
	"fmt"
	"io"

	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/observability"
)

// StreamMessage implements [ai.StreamProvider] for Anthropic's Messages API.
// It sends a streaming request (stream=true) and returns a [ai.ChatStream] that
// yields incremental deltas as SSE events arrive from the API.
//
// Pre-stream errors (missing API key, non-2xx HTTP response, network failure) are
// returned immediately as a non-nil error. Mid-stream errors (e.g., Anthropic
// "error" event, SSE parse failure) are yielded through the iterator.
//
// Anthropic SSE lifecycle:
//
//	message_start → content_block_start → content_block_delta(s) →
//	content_block_stop → message_delta → message_stop
func (provider *AnthropicProvider) StreamMessage(ctx context.Context, request ai.ChatRequest) (*ai.ChatStream, error) {
	// Enrich span / observer if observability is wired into the context.
	span := observability.SpanFromContext(ctx)
	observer := observability.ObserverFromContext(ctx)

	if span != nil {
		span.AddEvent(observability.EventLLMRequestStart)
		span.SetAttributes(
			observability.String(observability.AttrLLMProvider, "anthropic"),
			observability.String(observability.AttrLLMEndpoint, provider.baseURL),
			observability.String(observability.AttrLLMModel, request.Model),
			observability.Bool("llm.streaming", true),
		)
	}

	if observer != nil {
		observer.Trace(ctx, "Anthropic provider preparing streaming request",
			observability.String(observability.AttrLLMProvider, "anthropic"),
			observability.String(observability.AttrLLMEndpoint, provider.baseURL),
			observability.String(observability.AttrLLMModel, request.Model),
			observability.Int(observability.AttrRequestMessagesCount, len(request.Messages)),
			observability.Int(observability.AttrRequestToolsCount, len(request.Tools)),
		)
	}

	// Guard against missing credentials before making a network call.
	if provider.apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}

	streamURL := provider.baseURL + messagesEndpoint

	// Convert the generic request and enable streaming mode.
	anthropicReq, err := requestToAnthropic(request, provider.capabilities)
	if err != nil {
		return nil, fmt.Errorf("failed to build Anthropic request: %w", err)
	}
	anthropicReq.Stream = true

	// Send the streaming request — body is left open for SSE reading.
	// Pass empty apiKey so DoPostStream does not inject a Bearer token;
	// Anthropic authenticates via x-api-key (set inside buildHeaders).
	httpResponse, err := utils.DoPostStream(ctx, provider.client, streamURL, "", anthropicReq, provider.buildHeaders()...)
	if err != nil {
		if observer != nil {
			observer.Trace(ctx, "Streaming HTTP request failed", observability.Error(err))
		}
		return nil, err
	}

	// Build the SSE scanner that will read lines from the open response body.
	sseScanner := utils.NewSSEScanner(httpResponse.Body)

	// iteratorFunc reads SSE events and converts them to ai.StreamEvent values.
	// It maintains per-stream state for block type tracking, tool call indexing,
	// and accumulating token counts across multiple events.
	iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
		// Ensure the response body is closed when the iterator is exhausted or
		// the caller breaks out of the loop early.
		defer utils.CloseWithLog(httpResponse.Body)

		// --- Per-stream mutable state ---

		// toolCallCounter is incremented on each content_block_start of type
		// "tool_use", giving each tool call a unique zero-based index consistent
		// with the ai.ToolCallDelta.Index contract.
		toolCallCounter := 0

		// Token counts are spread across multiple events (message_start for
		// input tokens, message_delta for output tokens) so they are accumulated
		// and emitted together in a single StreamEventUsage event.
		inputTokens := 0
		outputTokens := 0
		cacheCreationTokens := 0
		cacheReadTokens := 0

		// finishReason is captured from "message_delta" and used when
		// "message_stop" triggers the StreamEventDone event.
		finishReason := ""

		for {
			// Respect context cancellation between SSE reads.
			if ctx.Err() != nil {
				yield(ai.StreamEvent{}, ctx.Err())
				return
			}

			payload, sseErr := sseScanner.Next()
			if sseErr == io.EOF {
				// Stream finished normally — no explicit done event needed here
				// because "message_stop" already emitted StreamEventDone.
				return
			}
			if sseErr != nil {
				yield(ai.StreamEvent{}, fmt.Errorf("SSE read error: %w", sseErr))
				return
			}

			// Parse the JSON payload into a typed stream event envelope.
			event, parseErr := unmarshalStreamEvent(payload)
			if parseErr != nil {
				yield(ai.StreamEvent{}, fmt.Errorf("failed to parse stream event: %w", parseErr))
				return
			}

			switch event.Type {

			case "message_start":
				// message_start carries the initial usage snapshot (input tokens
				// and any prompt-cache counters). Output tokens are always 0 here.
				if event.Message != nil {
					inputTokens = event.Message.Usage.InputTokens
					cacheCreationTokens = event.Message.Usage.CacheCreationInputTokens
					cacheReadTokens = event.Message.Usage.CacheReadInputTokens
				}
				// Do not emit an event; wait for message_delta to have full data.

			case "content_block_start":
				// content_block_start announces which kind of block is opening.
				// For tool_use blocks we emit the tool call header immediately
				// (ID + Name) because these fields are only present on this event,
				// not on the subsequent input_json_delta events.
				if event.ContentBlock == nil {
					continue
				}

				if event.ContentBlock.Type == "tool_use" {
					toolEvent := ai.StreamEvent{
						Type: ai.StreamEventToolCall,
						ToolCall: &ai.ToolCallDelta{
							Index: toolCallCounter,
							ID:    event.ContentBlock.ID,
							Name:  event.ContentBlock.Name,
						},
					}
					if !yield(toolEvent, nil) {
						return
					}
					toolCallCounter++
				}
				// "text" and "thinking" blocks: just record type, no event yet.

			case "content_block_delta":
				// content_block_delta delivers incremental content. Route to the
				// correct StreamEvent type based on the delta discriminator field.
				if event.Delta == nil {
					continue
				}

				switch event.Delta.Type {
				case "text_delta":
					if event.Delta.Text != "" {
						if !yield(ai.StreamEvent{
							Type:    ai.StreamEventContent,
							Content: event.Delta.Text,
						}, nil) {
							return
						}
					}

				case "thinking_delta":
					if event.Delta.Thinking != "" {
						if !yield(ai.StreamEvent{
							Type:      ai.StreamEventReasoning,
							Reasoning: event.Delta.Thinking,
						}, nil) {
							return
						}
					}

				case "input_json_delta":
					// input_json_delta carries incremental JSON for a tool call's
					// arguments. toolCallCounter-1 is the index of the currently
					// open tool_use block (incremented after the start event).
					if event.Delta.PartialJSON != "" {
						if !yield(ai.StreamEvent{
							Type: ai.StreamEventToolCall,
							ToolCall: &ai.ToolCallDelta{
								Index:     toolCallCounter - 1,
								Arguments: event.Delta.PartialJSON,
							},
						}, nil) {
							return
						}
					}
				}

			case "content_block_stop":
			// content_block_stop closes the current block. No action needed;
			// the next content_block_start will identify the new block type.

			case "message_delta":
				// message_delta carries the final output token count and stop reason.
				// Emit the consolidated usage event here so callers always receive
				// usage before the done event.
				if event.Usage != nil {
					outputTokens = event.Usage.OutputTokens
				}

				if event.Delta != nil && event.Delta.StopReason != "" {
					finishReason = event.Delta.StopReason
				}

				// Emit a single usage event that aggregates all token counters.
				totalTokens := inputTokens + outputTokens
				cachedTokens := cacheCreationTokens + cacheReadTokens

				if !yield(ai.StreamEvent{
					Type: ai.StreamEventUsage,
					Usage: &ai.Usage{
						PromptTokens:     inputTokens,
						CompletionTokens: outputTokens,
						TotalTokens:      totalTokens,
						CachedTokens:     cachedTokens,
					},
				}, nil) {
					return
				}

			case "message_stop":
				// message_stop is the terminal event. Emit the done event with the
				// normalised finish reason captured from message_delta.
				yield(ai.StreamEvent{
					Type:         ai.StreamEventDone,
					FinishReason: mapStopReason(finishReason),
				}, nil)
				return

			case "error":
				// Anthropic "error" events signal a server-side failure mid-stream.
				// Propagate as an iterator error so Collect() surfaces it properly.
				errMsg := "unknown stream error"
				if event.Error != nil {
					errMsg = event.Error.Message
				}
				yield(ai.StreamEvent{}, fmt.Errorf("anthropic stream error: %s", errMsg))
				return

			case "ping":
				// ping is a keep-alive event; nothing to yield.

			default:
				// Unknown event types are silently skipped for forward-compatibility
				// with future Anthropic SSE additions.
			}
		}
	}

	return ai.NewChatStream(iteratorFunc), nil
}
