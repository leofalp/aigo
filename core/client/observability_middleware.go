package client

import (
	"context"

	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/observability"
)

// NewObservabilityMiddleware creates a MiddlewareConfig that provides distributed
// tracing spans, structured metrics, and log events for every LLM request.
//
// The send middleware records a span from the moment the request enters the chain
// to when the response (or error) is returned. The stream middleware records the
// same span, but defers completion metrics until the stream iterator is fully
// consumed (StreamEventDone) or encounters an error.
//
// Both the span and the observer are injected into the context before calling
// next, so that provider implementations can retrieve them via
// [observability.SpanFromContext] and [observability.ObserverFromContext].
//
// The middleware is automatically prepended to the chain by [New] when
// [WithObserver] is provided, making it the outermost wrapper. This means
// it observes the final outcome — after any retry or timeout middleware — which is
// the correct behavior for end-to-end request metrics.
//
// Parameters:
//   - observer: the observability provider; must not be nil.
//   - defaultModel: model name used to label span and metric attributes when
//     the request's own Model field is empty.
func NewObservabilityMiddleware(observer observability.Provider, defaultModel string) MiddlewareConfig {
	return MiddlewareConfig{
		Send:   buildObsSend(observer, defaultModel),
		Stream: buildObsStream(observer, defaultModel),
	}
}

// buildObsSend constructs the send middleware that wraps each provider call
// with a tracing span and records success/error metrics and logs.
func buildObsSend(observer observability.Provider, defaultModel string) Middleware {
	return func(next SendFunc) SendFunc {
		return func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			model := effectiveModel(request.Model, defaultModel)

			// 1. Start span and enrich context so downstream providers can attach child spans.
			ctx, span := observer.StartSpan(ctx, observability.SpanClientSendMessage,
				observability.String(observability.AttrLLMModel, model),
			)
			ctx = observability.ContextWithSpan(ctx, span)
			ctx = observability.ContextWithObserver(ctx, observer)

			// 2. Emit a debug log at request start.
			observer.Debug(ctx, "llm send",
				observability.String(observability.AttrLLMModel, model),
				observability.Int(observability.AttrRequestMessagesCount, len(request.Messages)),
			)

			// 3. Time the provider call.
			timer := utils.NewTimer()
			response, err := next(ctx, request)
			timer.Stop()

			// 4. Handle error path.
			if err != nil {
				span.RecordError(err)
				span.SetStatus(observability.StatusError, "llm send failed")
				span.End()

				observer.Error(ctx, "llm send failed",
					observability.Error(err),
					observability.Duration(observability.AttrDuration, timer.GetDuration()),
					observability.String(observability.AttrLLMModel, model),
				)

				observer.Counter(observability.MetricClientRequestCount).Add(ctx, 1,
					observability.String(observability.AttrStatus, "error"),
					observability.String(observability.AttrLLMModel, model),
				)

				return nil, err
			}

			// 5. Record success metrics and log.
			recordObsSuccess(ctx, span, observer, response, timer, model)

			return response, nil
		}
	}
}

// buildObsStream constructs the stream middleware that wraps the returned
// ChatStream's iterator to defer metric recording until the stream completes.
func buildObsStream(observer observability.Provider, defaultModel string) StreamMiddleware {
	return func(next StreamFunc) StreamFunc {
		return func(ctx context.Context, request ai.ChatRequest) (*ai.ChatStream, error) {
			model := effectiveModel(request.Model, defaultModel)

			// 1. Start span and enrich context.
			ctx, span := observer.StartSpan(ctx, observability.SpanClientSendMessage,
				observability.String(observability.AttrLLMModel, model),
			)
			ctx = observability.ContextWithSpan(ctx, span)
			ctx = observability.ContextWithObserver(ctx, observer)

			// 2. Emit a debug log at stream start.
			observer.Debug(ctx, "llm stream",
				observability.String(observability.AttrLLMModel, model),
				observability.Int(observability.AttrRequestMessagesCount, len(request.Messages)),
			)

			// 3. Time the call that initiates the stream.
			timer := utils.NewTimer()
			stream, err := next(ctx, request)
			if err != nil {
				timer.Stop()

				span.RecordError(err)
				span.SetStatus(observability.StatusError, "llm stream failed")
				span.End()

				observer.Error(ctx, "llm stream failed",
					observability.Error(err),
					observability.Duration(observability.AttrDuration, timer.GetDuration()),
					observability.String(observability.AttrLLMModel, model),
				)

				observer.Counter(observability.MetricClientRequestCount).Add(ctx, 1,
					observability.String(observability.AttrStatus, "error"),
					observability.String(observability.AttrLLMModel, model),
				)

				return nil, err
			}

			// 4. Wrap the iterator so we can observe it as it drains.
			wrapped := wrapStreamWithObservability(ctx, stream, span, observer, timer, model)
			return wrapped, nil
		}
	}
}

// wrapStreamWithObservability returns a new ChatStream whose iterator emits all
// events unchanged but records observability data when the stream ends normally
// (StreamEventDone), is abandoned (caller breaks early), or encounters an error.
func wrapStreamWithObservability(
	ctx context.Context,
	stream *ai.ChatStream,
	span observability.Span,
	observer observability.Provider,
	timer *utils.Timer,
	model string,
) *ai.ChatStream {
	iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
		var usage *ai.Usage
		var finishReason string

		for event, err := range stream.Iter() {
			if err != nil {
				timer.Stop()

				span.RecordError(err)
				span.SetStatus(observability.StatusError, "llm stream failed")
				span.End()

				observer.Error(ctx, "llm stream failed",
					observability.Error(err),
					observability.Duration(observability.AttrDuration, timer.GetDuration()),
					observability.String(observability.AttrLLMModel, model),
				)

				observer.Counter(observability.MetricClientRequestCount).Add(ctx, 1,
					observability.String(observability.AttrStatus, "error"),
					observability.String(observability.AttrLLMModel, model),
				)

				yield(event, err)
				return
			}

			// Capture metadata for the completion log entry.
			if event.Type == ai.StreamEventUsage && event.Usage != nil {
				usage = event.Usage
			}

			if event.Type == ai.StreamEventDone {
				finishReason = event.FinishReason
			}

			if !yield(event, nil) {
				// Caller broke out early — record what we have so far.
				timer.Stop()
				span.SetStatus(observability.StatusOK, "llm stream abandoned")
				span.End()

				observer.Info(ctx, "llm stream abandoned",
					observability.String(observability.AttrLLMModel, model),
					observability.Duration(observability.AttrDuration, timer.GetDuration()),
				)

				return
			}

			if event.Type == ai.StreamEventDone {
				break
			}
		}

		// Stream completed normally.
		timer.Stop()

		// Build a synthetic response to reuse the shared success recorder.
		syntheticResponse := &ai.ChatResponse{
			Model:        model,
			FinishReason: finishReason,
			Usage:        usage,
		}

		recordObsSuccess(ctx, span, observer, syntheticResponse, timer, model)
	}

	return ai.NewChatStream(iteratorFunc)
}

// recordObsSuccess writes all success-path observability data: duration histogram,
// request counter, token counters, span attributes, a structured INFO log, and
// then ends the span.
func recordObsSuccess(
	ctx context.Context,
	span observability.Span,
	observer observability.Provider,
	response *ai.ChatResponse,
	timer *utils.Timer,
	model string,
) {
	elapsed := timer.GetDuration()

	// Metrics
	observer.Histogram(observability.MetricClientRequestDuration).Record(ctx, elapsed.Seconds(),
		observability.String(observability.AttrLLMModel, model),
	)

	observer.Counter(observability.MetricClientRequestCount).Add(ctx, 1,
		observability.String(observability.AttrStatus, "success"),
		observability.String(observability.AttrLLMModel, model),
	)

	// Log attributes (always present)
	logAttrs := []observability.Attribute{
		observability.String(observability.AttrLLMModel, model),
		observability.String(observability.AttrLLMFinishReason, response.FinishReason),
		observability.Duration(observability.AttrDuration, elapsed),
		observability.Int(observability.AttrClientToolCalls, len(response.ToolCalls)),
	}

	// Token counters and span attributes (when usage is available)
	if response.Usage != nil {
		observer.Counter(observability.MetricClientTokensTotal).Add(ctx, int64(response.Usage.TotalTokens),
			observability.String(observability.AttrLLMModel, model),
		)
		observer.Counter(observability.MetricClientTokensPrompt).Add(ctx, int64(response.Usage.PromptTokens),
			observability.String(observability.AttrLLMModel, model),
		)
		observer.Counter(observability.MetricClientTokensCompletion).Add(ctx, int64(response.Usage.CompletionTokens),
			observability.String(observability.AttrLLMModel, model),
		)

		span.SetAttributes(
			observability.Int(observability.AttrLLMTokensTotal, response.Usage.TotalTokens),
			observability.Int(observability.AttrLLMTokensPrompt, response.Usage.PromptTokens),
			observability.Int(observability.AttrLLMTokensCompletion, response.Usage.CompletionTokens),
		)

		logAttrs = append(logAttrs,
			observability.Int(observability.AttrLLMTokensPrompt, response.Usage.PromptTokens),
			observability.Int(observability.AttrLLMTokensCompletion, response.Usage.CompletionTokens),
			observability.Int(observability.AttrLLMTokensTotal, response.Usage.TotalTokens),
		)
	}

	// Add tool call names if present
	if len(response.ToolCalls) > 0 {
		toolNames := make([]string, len(response.ToolCalls))
		for i, toolCall := range response.ToolCalls {
			toolNames[i] = toolCall.Function.Name
		}

		logAttrs = append(logAttrs, observability.StringSlice("tool_calls", toolNames))
	}

	// Add response content preview if present
	if response.Content != "" {
		logAttrs = append(logAttrs,
			observability.String("response", utils.TruncateString(response.Content, 100)),
		)
	}

	observer.Info(ctx, "llm send completed", logAttrs...)

	span.SetStatus(observability.StatusOK, "success")
	span.End()
}

// effectiveModel returns the request-level model when set, falling back to the
// client's configured default. Both being empty is valid (provider chooses).
func effectiveModel(requestModel, defaultModel string) string {
	if requestModel != "" {
		return requestModel
	}

	return defaultModel
}
