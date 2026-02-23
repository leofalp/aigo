package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/ai"
)

// LogLevel controls how much detail the logging middleware emits per request.
type LogLevel int

const (
	// LogLevelMinimal logs only the model name, total duration, and token counts.
	// Use this when you want lightweight audit trails without noise.
	LogLevelMinimal LogLevel = iota

	// LogLevelStandard logs everything in Minimal plus the message count and
	// finish reason. This is the recommended default for most applications.
	LogLevelStandard

	// LogLevelVerbose logs everything in Standard plus the first message content
	// and the full response content, each truncated to 500 characters.
	//
	// WARNING: DO NOT use LogLevelVerbose in production. It will log raw prompt
	// and response text, which may contain sensitive user data, secrets, or PII.
	// It is intended solely for local debugging and development.
	LogLevelVerbose
)

// truncateLen is the maximum content length included in verbose log output.
const truncateLen = 500

// NewLoggingMiddleware creates a MiddlewareConfig that emits structured slog
// log entries before and after every provider call. Both synchronous and
// streaming calls are covered: for streams the completion entry is emitted once
// the iterator is fully consumed (StreamEventDone or error).
//
// The logger parameter must not be nil. Use slog.Default() if you have not
// configured a custom logger.
func NewLoggingMiddleware(logger *slog.Logger, level LogLevel) client.MiddlewareConfig {
	return client.MiddlewareConfig{
		Send:   buildSendLogging(logger, level),
		Stream: buildStreamLogging(logger, level),
	}
}

// buildSendLogging constructs the send middleware that logs request/response pairs.
func buildSendLogging(logger *slog.Logger, level LogLevel) client.Middleware {
	return func(next client.SendFunc) client.SendFunc {
		return func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			logger.InfoContext(ctx, "llm send",
				buildRequestAttrs(request, level)...,
			)

			start := time.Now()
			response, err := next(ctx, request)
			elapsed := time.Since(start)

			if err != nil {
				logger.ErrorContext(ctx, "llm send failed",
					slog.String("model", request.Model),
					slog.Duration("duration", elapsed),
					slog.String("error", err.Error()),
				)
				return nil, err
			}

			logger.InfoContext(ctx, "llm send completed",
				buildResponseAttrs(response, elapsed, level)...,
			)

			return response, nil
		}
	}
}

// buildStreamLogging constructs the stream middleware that logs stream start and
// wraps the iterator to log completion or error at the end of the stream.
func buildStreamLogging(logger *slog.Logger, level LogLevel) client.StreamMiddleware {
	return func(next client.StreamFunc) client.StreamFunc {
		return func(ctx context.Context, request ai.ChatRequest) (*ai.ChatStream, error) {
			logger.InfoContext(ctx, "llm stream",
				buildRequestAttrs(request, level)...,
			)

			start := time.Now()
			stream, err := next(ctx, request)
			if err != nil {
				elapsed := time.Since(start)
				logger.ErrorContext(ctx, "llm stream failed",
					slog.String("model", request.Model),
					slog.Duration("duration", elapsed),
					slog.String("error", err.Error()),
				)
				return nil, err
			}

			// Wrap the stream iterator so we can log when it finishes.
			wrapped := wrapStreamWithLogging(ctx, stream, logger, request.Model, level, start)
			return wrapped, nil
		}
	}
}

// wrapStreamWithLogging returns a new ChatStream whose iterator logs a
// completion entry when the stream ends normally, or an error entry on failure.
func wrapStreamWithLogging(
	ctx context.Context,
	stream *ai.ChatStream,
	logger *slog.Logger,
	model string,
	level LogLevel,
	start time.Time,
) *ai.ChatStream {
	iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
		var finishReason string
		var usage *ai.Usage

		for event, err := range stream.Iter() {
			if err != nil {
				elapsed := time.Since(start)
				logger.ErrorContext(ctx, "llm stream failed",
					slog.String("model", model),
					slog.Duration("duration", elapsed),
					slog.String("error", err.Error()),
				)
				yield(event, err)
				return
			}

			// Capture metadata carried on the stream for the completion log entry.
			if event.Type == ai.StreamEventUsage && event.Usage != nil {
				usage = event.Usage
			}

			if event.Type == ai.StreamEventDone {
				finishReason = event.FinishReason
			}

			if !yield(event, nil) {
				// Caller broke out of the range loop early — log what we have.
				elapsed := time.Since(start)
				logger.InfoContext(ctx, "llm stream abandoned",
					slog.String("model", model),
					slog.Duration("duration", elapsed),
				)
				return
			}

			if event.Type == ai.StreamEventDone {
				break
			}
		}

		elapsed := time.Since(start)

		attrs := []any{
			slog.String("model", model),
			slog.Duration("duration", elapsed),
		}

		if level >= LogLevelStandard && finishReason != "" {
			attrs = append(attrs, slog.String("finish_reason", finishReason))
		}

		if usage != nil {
			attrs = append(attrs,
				slog.Int("prompt_tokens", usage.PromptTokens),
				slog.Int("completion_tokens", usage.CompletionTokens),
				slog.Int("total_tokens", usage.TotalTokens),
			)
		}

		logger.InfoContext(ctx, "llm stream completed", attrs...)
	}

	return ai.NewChatStream(iteratorFunc)
}

// buildRequestAttrs returns slog attributes for an outgoing chat request,
// expanding detail according to the requested verbosity level.
func buildRequestAttrs(request ai.ChatRequest, level LogLevel) []any {
	attrs := []any{
		slog.String("model", request.Model),
	}

	if level >= LogLevelStandard {
		attrs = append(attrs, slog.Int("message_count", len(request.Messages)))
	}

	if level >= LogLevelVerbose && len(request.Messages) > 0 {
		first := request.Messages[0]
		attrs = append(attrs,
			slog.String("first_message_role", string(first.Role)),
			slog.String("first_message_content", utils.TruncateString(first.Content, truncateLen)),
		)
	}

	return attrs
}

// buildResponseAttrs returns slog attributes for a completed chat response,
// expanding detail according to the requested verbosity level.
func buildResponseAttrs(response *ai.ChatResponse, elapsed time.Duration, level LogLevel) []any {
	attrs := []any{
		slog.String("model", response.Model),
		slog.Duration("duration", elapsed),
	}

	if response.Usage != nil {
		attrs = append(attrs,
			slog.Int("prompt_tokens", response.Usage.PromptTokens),
			slog.Int("completion_tokens", response.Usage.CompletionTokens),
			slog.Int("total_tokens", response.Usage.TotalTokens),
		)
	}

	if level >= LogLevelStandard && response.FinishReason != "" {
		attrs = append(attrs, slog.String("finish_reason", response.FinishReason))
	}

	if level >= LogLevelVerbose && response.Content != "" {
		attrs = append(attrs,
			slog.String("response_content", utils.TruncateString(response.Content, truncateLen)),
		)
	}

	return attrs
}
