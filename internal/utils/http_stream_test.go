package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---- SSEScanner tests -------------------------------------------------------

// TestSSEScanner_SingleEvent verifies that a simple "data: <payload>\n\n"
// produces exactly one payload and then io.EOF.
func TestSSEScanner_SingleEvent_ReturnsSinglePayload(t *testing.T) {
	input := "data: hello\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	payload, err := scanner.Next()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if payload != "hello" {
		t.Errorf("expected payload %q, got %q", "hello", payload)
	}

	_, err = scanner.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF after last event, got %v", err)
	}
}

// TestSSEScanner_MultipleEvents verifies that multiple events separated by
// blank lines are returned in order.
func TestSSEScanner_MultipleEvents_ReturnsInOrder(t *testing.T) {
	input := "data: first\n\ndata: second\n\ndata: third\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	expectedPayloads := []string{"first", "second", "third"}
	for _, expected := range expectedPayloads {
		payload, err := scanner.Next()
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if payload != expected {
			t.Errorf("expected %q, got %q", expected, payload)
		}
	}

	_, err := scanner.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

// TestSSEScanner_MultiLineDataEvent verifies that consecutive "data:" lines
// within a single event are joined with newlines into a single payload.
func TestSSEScanner_MultiLineDataEvent_JoinsWithNewline(t *testing.T) {
	input := "data: line1\ndata: line2\ndata: line3\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	payload, err := scanner.Next()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	expected := "line1\nline2\nline3"
	if payload != expected {
		t.Errorf("expected %q, got %q", expected, payload)
	}
}

// TestSSEScanner_SkipsComments verifies that lines starting with ":" are
// treated as SSE comments and ignored.
func TestSSEScanner_SkipsComments_ReturnsOnlyDataEvents(t *testing.T) {
	input := ": this is a comment\ndata: real payload\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	payload, err := scanner.Next()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if payload != "real payload" {
		t.Errorf("expected %q, got %q", "real payload", payload)
	}
}

// TestSSEScanner_DoneSentinel verifies that a "data: [DONE]" line causes
// Next() to return io.EOF immediately (OpenAI convention).
func TestSSEScanner_DoneSentinel_ReturnsEOF(t *testing.T) {
	input := "data: before\n\ndata: [DONE]\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	// First event should succeed
	_, err := scanner.Next()
	if err != nil {
		t.Fatalf("expected nil error on first event, got %v", err)
	}

	// [DONE] should produce io.EOF
	_, err = scanner.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF on [DONE], got %v", err)
	}
}

// TestSSEScanner_EmptyStream verifies that an empty input returns io.EOF
// immediately without panicking.
func TestSSEScanner_EmptyStream_ReturnsEOF(t *testing.T) {
	scanner := NewSSEScanner(strings.NewReader(""))

	_, err := scanner.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF for empty stream, got %v", err)
	}
}

// TestSSEScanner_TrailingDataWithoutFinalBlankLine verifies that data lines
// present at the end of the stream (with no trailing blank line) are still
// returned by Next() rather than silently dropped.
func TestSSEScanner_TrailingDataWithoutFinalBlankLine_ReturnsPayload(t *testing.T) {
	// No trailing "\n\n" — the stream just ends after the data line.
	input := "data: no-trailing-blank"
	scanner := NewSSEScanner(strings.NewReader(input))

	payload, err := scanner.Next()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if payload != "no-trailing-blank" {
		t.Errorf("expected %q, got %q", "no-trailing-blank", payload)
	}
}

// TestSSEScanner_WhitespaceTrimming verifies that leading/trailing whitespace
// after the "data:" prefix is trimmed from the payload.
func TestSSEScanner_WhitespaceTrimming_TrimsPayload(t *testing.T) {
	input := "data:   padded value   \n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	payload, err := scanner.Next()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if payload != "padded value" {
		t.Errorf("expected %q, got %q", "padded value", payload)
	}
}

// TestSSEScanner_SkipsNonDataFields verifies that SSE fields other than
// "data:" (e.g. "event:", "id:", "retry:") are silently ignored.
func TestSSEScanner_SkipsNonDataFields_IgnoresOtherFields(t *testing.T) {
	input := "event: update\nid: 42\nretry: 3000\ndata: payload\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	payload, err := scanner.Next()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if payload != "payload" {
		t.Errorf("expected %q, got %q", "payload", payload)
	}
}

// TestSSEScanner_ConsecutiveBlankLines verifies that multiple consecutive
// blank lines between events do not cause duplicate or empty payloads.
func TestSSEScanner_ConsecutiveBlankLines_SkipsEmpty(t *testing.T) {
	input := "data: event1\n\n\n\ndata: event2\n\n"
	scanner := NewSSEScanner(strings.NewReader(input))

	payload1, err := scanner.Next()
	if err != nil {
		t.Fatalf("expected nil error on first call, got %v", err)
	}
	if payload1 != "event1" {
		t.Errorf("expected %q, got %q", "event1", payload1)
	}

	payload2, err := scanner.Next()
	if err != nil {
		t.Fatalf("expected nil error on second call, got %v", err)
	}
	if payload2 != "event2" {
		t.Errorf("expected %q, got %q", "event2", payload2)
	}

	_, err = scanner.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

// ---- DoPostStream tests -----------------------------------------------------

// TestDoPostStream_SuccessResponse_ReturnsOpenBody verifies that a 200 response
// leaves the body open for the caller to read from (SSE consumption pattern).
func TestDoPostStream_SuccessResponse_ReturnsOpenBody(t *testing.T) {
	ssePayload := "data: chunk1\n\ndata: [DONE]\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, ssePayload)
	}))
	defer server.Close()

	response, err := DoPostStream(context.Background(), server.Client(), server.URL, "test-key", map[string]string{"q": "test"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer CloseWithLog(response.Body)

	// Body must still be readable — consume via SSEScanner
	scanner := NewSSEScanner(response.Body)
	payload, scanErr := scanner.Next()
	if scanErr != nil {
		t.Fatalf("expected nil error reading SSE, got %v", scanErr)
	}
	if payload != "chunk1" {
		t.Errorf("expected %q, got %q", "chunk1", payload)
	}
}

// TestDoPostStream_NonTwoxxResponse_ReturnsError verifies that a non-2xx
// HTTP status causes DoPostStream to return an error with the status code.
func TestDoPostStream_NonTwoxxResponse_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := DoPostStream(context.Background(), server.Client(), server.URL, "test-key", map[string]string{})
	if err == nil {
		t.Fatal("expected error for non-2xx response, got nil")
	}

	// Error should mention the status code
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("expected error to contain status code 429, got: %v", err)
	}
}

// TestDoPostStream_ServerError_ReturnsError verifies that a 500 response is
// treated as an error and the body contents are included in the error message.
func TestDoPostStream_ServerError_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := DoPostStream(context.Background(), server.Client(), server.URL, "", map[string]string{})
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain status 500, got: %v", err)
	}
}

// TestDoPostStream_ContextCancellation_ReturnsError verifies that a
// pre-cancelled context causes DoPostStream to return an error immediately.
func TestDoPostStream_ContextCancellation_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This handler will never be reached if context is already cancelled.
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately before the request

	_, err := DoPostStream(cancelledCtx, server.Client(), server.URL, "", map[string]string{})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// TestDoPostStream_NetworkError_ReturnsError verifies that an unreachable
// server causes DoPostStream to return a wrapped error.
func TestDoPostStream_NetworkError_ReturnsError(t *testing.T) {
	// Point to a port that is guaranteed not to be listening.
	_, err := DoPostStream(context.Background(), nil, "http://127.0.0.1:1", "", map[string]string{})
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

// TestDoPostStream_SetsAuthHeader_WithAPIKey verifies that when an API key is
// provided the Authorization header is sent as a Bearer token.
func TestDoPostStream_SetsAuthHeader_WithAPIKey(t *testing.T) {
	const expectedKey = "supersecret"
	var capturedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	response, err := DoPostStream(context.Background(), server.Client(), server.URL, expectedKey, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	CloseWithLog(response.Body)

	expected := "Bearer " + expectedKey
	if capturedAuth != expected {
		t.Errorf("expected Authorization header %q, got %q", expected, capturedAuth)
	}
}

// TestDoPostStream_CustomHeader_OverridesDefault verifies that a HeaderOption
// is applied to the outgoing request, overriding any default header value.
func TestDoPostStream_CustomHeader_OverridesDefault(t *testing.T) {
	const customHeaderKey = "x-custom-provider-key"
	const customHeaderValue = "provider-token-123"
	var capturedHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get(customHeaderKey)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	response, err := DoPostStream(
		context.Background(),
		server.Client(),
		server.URL,
		"",
		map[string]string{},
		HeaderOption{Key: customHeaderKey, Value: customHeaderValue},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	CloseWithLog(response.Body)

	if capturedHeader != customHeaderValue {
		t.Errorf("expected custom header %q, got %q", customHeaderValue, capturedHeader)
	}
}
