package middleware

import (
	"bytes"
	"context"
	"errors"
	"iter"
	"log/slog"
	"strings"
	"testing"

	"github.com/leofalp/aigo/providers/ai"
)

// ========== Test logger helpers ==========

// testLogger creates an slog.Logger that writes to a *bytes.Buffer so tests
// can inspect emitted log lines without capturing os.Stderr.
func testLogger(buf *bytes.Buffer) *slog.Logger {
	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler)
}

// logContains returns true if the log buffer contains the given substring.
func logContains(buf *bytes.Buffer, substr string) bool {
	return strings.Contains(buf.String(), substr)
}

// ========== Synchronous send tests ==========

// TestLoggingMiddleware_Send_Minimal verifies that at LogLevelMinimal only the
// model and duration attributes appear in the success log (no message_count,
// no finish_reason, no content).
func TestLoggingMiddleware_Send_Minimal(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)

	mw := NewLoggingMiddleware(logger, LogLevelMinimal)

	next := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		return &ai.ChatResponse{
			Model:        "test-model",
			Content:      "hello world",
			FinishReason: "stop",
			Usage:        &ai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}, nil
	}

	chain := mw.Send(next)
	_, err := chain(context.Background(), ai.ChatRequest{Model: "test-model", Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Should include model and token counts.
	if !logContains(buf, "test-model") {
		t.Errorf("expected model in log, got:\n%s", output)
	}
	if !logContains(buf, "prompt_tokens") {
		t.Errorf("expected prompt_tokens in log, got:\n%s", output)
	}

	// Should NOT include message_count or finish_reason at Minimal level.
	if logContains(buf, "message_count") {
		t.Errorf("did not expect message_count at LogLevelMinimal, got:\n%s", output)
	}
	if logContains(buf, "finish_reason") {
		t.Errorf("did not expect finish_reason at LogLevelMinimal, got:\n%s", output)
	}
	// Should NOT include response content at Minimal level.
	if logContains(buf, "response_content") {
		t.Errorf("did not expect response_content at LogLevelMinimal, got:\n%s", output)
	}
}

// TestLoggingMiddleware_Send_Standard verifies that at LogLevelStandard the log
// includes message_count and finish_reason in addition to Minimal fields.
func TestLoggingMiddleware_Send_Standard(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)

	mw := NewLoggingMiddleware(logger, LogLevelStandard)

	next := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		return &ai.ChatResponse{
			Model:        "test-model",
			Content:      "hello",
			FinishReason: "stop",
		}, nil
	}

	chain := mw.Send(next)
	_, err := chain(context.Background(), ai.ChatRequest{
		Model:    "test-model",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !logContains(buf, "message_count") {
		t.Errorf("expected message_count in log, got:\n%s", buf.String())
	}
	if !logContains(buf, "finish_reason") {
		t.Errorf("expected finish_reason in log, got:\n%s", buf.String())
	}
	// No response_content at Standard.
	if logContains(buf, "response_content") {
		t.Errorf("did not expect response_content at LogLevelStandard, got:\n%s", buf.String())
	}
}

// TestLoggingMiddleware_Send_Verbose verifies that at LogLevelVerbose the log
// includes the truncated response content and first message content.
func TestLoggingMiddleware_Send_Verbose(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)

	mw := NewLoggingMiddleware(logger, LogLevelVerbose)

	next := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		return &ai.ChatResponse{
			Model:        "test-model",
			Content:      "verbose response",
			FinishReason: "stop",
		}, nil
	}

	chain := mw.Send(next)
	_, err := chain(context.Background(), ai.ChatRequest{
		Model:    "test-model",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "verbose request"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !logContains(buf, "first_message_content") {
		t.Errorf("expected first_message_content in log, got:\n%s", buf.String())
	}
	if !logContains(buf, "response_content") {
		t.Errorf("expected response_content in log, got:\n%s", buf.String())
	}
}

// TestLoggingMiddleware_Send_ErrorPath verifies that when the provider returns
// an error the middleware logs an error entry and propagates the error.
func TestLoggingMiddleware_Send_ErrorPath(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)

	mw := NewLoggingMiddleware(logger, LogLevelStandard)

	providerErr := errors.New("provider unavailable")
	next := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatResponse, error) {
		return nil, providerErr
	}

	chain := mw.Send(next)
	_, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})

	if !errors.Is(err, providerErr) {
		t.Errorf("expected providerErr, got %v", err)
	}

	if !logContains(buf, "ERROR") {
		t.Errorf("expected ERROR level log on failure, got:\n%s", buf.String())
	}
	if !logContains(buf, "provider unavailable") {
		t.Errorf("expected error message in log, got:\n%s", buf.String())
	}
}

// TestLoggingMiddleware_Send_NoNilStreamField verifies both Send and Stream
// fields are non-nil for the logging middleware (unlike retry).
func TestLoggingMiddleware_Send_NoNilStreamField(t *testing.T) {
	mw := NewLoggingMiddleware(slog.Default(), LogLevelMinimal)
	if mw.Send == nil {
		t.Error("expected non-nil Send field")
	}
	if mw.Stream == nil {
		t.Error("expected non-nil Stream field")
	}
}

// ========== Streaming tests ==========

// makeSimpleStreamFunc returns a StreamFunc that emits a content event, a usage
// event, and a done event in sequence.
func makeSimpleStreamFunc(content string) func(context.Context, ai.ChatRequest) (*ai.ChatStream, error) {
	return func(_ context.Context, _ ai.ChatRequest) (*ai.ChatStream, error) {
		iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
			if !yield(ai.StreamEvent{Type: ai.StreamEventContent, Content: content}, nil) {
				return
			}
			if !yield(ai.StreamEvent{
				Type:  ai.StreamEventUsage,
				Usage: &ai.Usage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8},
			}, nil) {
				return
			}
			yield(ai.StreamEvent{Type: ai.StreamEventDone, FinishReason: "stop"}, nil)
		}
		return ai.NewChatStream(iter.Seq2[ai.StreamEvent, error](iteratorFunc)), nil
	}
}

// TestLoggingMiddleware_Stream_LogsStartAndCompletion verifies that streaming
// emits start and completion log entries after the stream is fully consumed.
func TestLoggingMiddleware_Stream_LogsStartAndCompletion(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)

	mw := NewLoggingMiddleware(logger, LogLevelStandard)
	chain := mw.Stream(makeSimpleStreamFunc("hello"))

	stream, err := chain(context.Background(), ai.ChatRequest{Model: "test-model", Messages: []ai.Message{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Consume the stream — the completion log is emitted during iteration.
	resp, collectErr := stream.Collect()
	if collectErr != nil {
		t.Fatalf("Collect error: %v", collectErr)
	}
	if resp.Content != "hello" {
		t.Errorf("expected 'hello', got %q", resp.Content)
	}

	output := buf.String()

	if !logContains(buf, "llm stream") {
		t.Errorf("expected start log entry, got:\n%s", output)
	}
	if !logContains(buf, "llm stream completed") {
		t.Errorf("expected completion log entry, got:\n%s", output)
	}
}

// TestLoggingMiddleware_Stream_Standard_IncludesFinishReason verifies that at
// LogLevelStandard the completion log entry includes the finish reason.
func TestLoggingMiddleware_Stream_Standard_IncludesFinishReason(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)

	mw := NewLoggingMiddleware(logger, LogLevelStandard)
	chain := mw.Stream(makeSimpleStreamFunc("hi"))

	stream, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _ = stream.Collect()

	if !logContains(buf, "finish_reason") {
		t.Errorf("expected finish_reason in stream completion log, got:\n%s", buf.String())
	}
}

// TestLoggingMiddleware_Stream_ErrorPath verifies that a mid-stream error is
// logged as an error entry and returned to the caller.
func TestLoggingMiddleware_Stream_ErrorPath(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)

	mw := NewLoggingMiddleware(logger, LogLevelStandard)

	streamErr := errors.New("mid-stream failure")
	errStreamFunc := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatStream, error) {
		iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
			yield(ai.StreamEvent{}, streamErr)
		}
		return ai.NewChatStream(iter.Seq2[ai.StreamEvent, error](iteratorFunc)), nil
	}

	chain := mw.Stream(errStreamFunc)
	stream, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("unexpected pre-stream error: %v", err)
	}

	_, collectErr := stream.Collect()
	if !errors.Is(collectErr, streamErr) {
		t.Errorf("expected streamErr, got %v", collectErr)
	}

	if !logContains(buf, "ERROR") {
		t.Errorf("expected ERROR log on mid-stream failure, got:\n%s", buf.String())
	}
	if !logContains(buf, "mid-stream failure") {
		t.Errorf("expected error message in log, got:\n%s", buf.String())
	}
}

// TestLoggingMiddleware_Stream_PreStreamError verifies that when the provider
// returns an error before streaming begins, the middleware logs and propagates it.
func TestLoggingMiddleware_Stream_PreStreamError(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)

	mw := NewLoggingMiddleware(logger, LogLevelStandard)

	preErr := errors.New("auth failure")
	errStreamFunc := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatStream, error) {
		return nil, preErr
	}

	chain := mw.Stream(errStreamFunc)
	_, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})

	if !errors.Is(err, preErr) {
		t.Errorf("expected preErr, got %v", err)
	}

	if !logContains(buf, "ERROR") {
		t.Errorf("expected ERROR log on pre-stream failure, got:\n%s", buf.String())
	}
}

// TestLoggingMiddleware_Stream_TokensLogged verifies that token usage captured
// from a StreamEventUsage event appears in the completion log.
func TestLoggingMiddleware_Stream_TokensLogged(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)

	mw := NewLoggingMiddleware(logger, LogLevelMinimal)
	chain := mw.Stream(makeSimpleStreamFunc("token test"))

	stream, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _ = stream.Collect()

	if !logContains(buf, "total_tokens") {
		t.Errorf("expected total_tokens in stream log, got:\n%s", buf.String())
	}
}

// TestLoggingMiddleware_Stream_AbandonedLog verifies that breaking out of the
// stream early results in a "llm stream abandoned" log entry.
func TestLoggingMiddleware_Stream_AbandonedLog(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)

	mw := NewLoggingMiddleware(logger, LogLevelMinimal)

	// Stream that emits many events so the caller can break early.
	longStreamFunc := func(_ context.Context, _ ai.ChatRequest) (*ai.ChatStream, error) {
		iteratorFunc := func(yield func(ai.StreamEvent, error) bool) {
			for i := 0; i < 10; i++ {
				if !yield(ai.StreamEvent{Type: ai.StreamEventContent, Content: "x"}, nil) {
					return
				}
			}
			yield(ai.StreamEvent{Type: ai.StreamEventDone}, nil)
		}
		return ai.NewChatStream(iter.Seq2[ai.StreamEvent, error](iteratorFunc)), nil
	}

	chain := mw.Stream(longStreamFunc)
	stream, err := chain(context.Background(), ai.ChatRequest{Model: "test-model"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Break after the first event.
	for range stream.Iter() {
		break
	}

	// The "abandoned" log is written synchronously inside the iterator before it
	// returns, so no sleep or channel synchronization is needed here.
	if !logContains(buf, "abandoned") {
		t.Errorf("expected 'abandoned' in log after early break, got:\n%s", buf.String())
	}
}
