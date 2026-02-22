package utils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/leofalp/aigo/providers/observability"
)

// DoPostStream performs an HTTP POST request and returns the raw response with body
// left open for SSE reading. The caller is responsible for closing the response body
// when done reading. On error paths the body is read and closed before returning.
//
// This follows the same pattern as DoPostSync but does not consume the response body,
// enabling streaming consumption via SSEScanner.
func DoPostStream(ctx context.Context, client *http.Client, url string, apiKey string, body any, headers ...HeaderOption) (*http.Response, error) {
	// Get observer from context if available
	span := observability.SpanFromContext(ctx)

	httpClient := client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("error marshaling body: %w", err)
	}

	if span != nil {
		span.AddEvent("http.stream_request.prepared",
			observability.String(observability.AttrHTTPMethod, "POST"),
			observability.String(observability.AttrHTTPURL, url),
			observability.Int(observability.AttrHTTPRequestBodySize, len(jsonBody)),
		)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Apply custom headers (can override Authorization if needed)
	for _, header := range headers {
		req.Header.Set(header.Key, header.Value)
	}

	requestStart := time.Now()
	response, err := httpClient.Do(req)
	requestDuration := time.Since(requestStart)

	if err != nil {
		if span != nil {
			span.AddEvent("http.stream_request.error",
				observability.Error(err),
				observability.Duration("http.request.duration", requestDuration),
			)
		}
		return response, fmt.Errorf("error sending stream request: %w", err)
	}

	// For non-2xx responses, read the body and close it before returning the error
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer CloseWithLog(response.Body)
		// Cap body reads to maxResponseBodySize to prevent unbounded memory allocation.
		errorBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBodySize))
		if readErr != nil {
			return response, fmt.Errorf("non-2xx status %d (failed to read body: %v)", response.StatusCode, readErr)
		}
		return response, fmt.Errorf("non-2xx status %d: %s", response.StatusCode, string(errorBody))
	}

	if span != nil {
		span.AddEvent("http.stream_response.started",
			observability.Int(observability.AttrHTTPStatusCode, response.StatusCode),
			observability.Duration("http.request.duration", requestDuration),
		)
	}

	return response, nil
}

// maxSSELineSize is the maximum size of a single SSE line (1 MB).
// The default bufio.Scanner limit is 64 KiB, which is too small for
// large SSE events such as tool-call arguments or long completions
// from OpenAI/Gemini. If a line exceeds this limit the scanner will
// return a wrapped bufio.ErrTooLong via the Next() error path.
const maxSSELineSize = 1 * 1024 * 1024

// maxResponseBodySize is the maximum response body size (10 MB). Enforced via
// io.LimitReader to prevent unbounded memory allocation from rogue responses.
const maxResponseBodySize int64 = 10 * 1024 * 1024

// SSEScanner reads Server-Sent Events (SSE) from an io.Reader.
// It handles multi-line data fields, skips comments and empty lines,
// and detects the [DONE] sentinel used by OpenAI-compatible APIs.
type SSEScanner struct {
	scanner *bufio.Scanner
}

// NewSSEScanner creates an SSEScanner that reads SSE events from the given reader.
// The scanner supports individual SSE lines up to maxSSELineSize (1 MB). Lines
// exceeding this limit will cause Next() to return an error wrapping bufio.ErrTooLong.
func NewSSEScanner(reader io.Reader) *SSEScanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSELineSize)
	return &SSEScanner{
		scanner: scanner,
	}
}

// Next returns the next SSE data payload as a string.
// It skips empty lines and comment lines (starting with ':').
// Returns io.EOF when no more events are available.
// Returns io.EOF when the [DONE] sentinel is encountered.
//
// Multi-line data fields (multiple consecutive "data:" lines) are joined
// with newlines into a single payload string.
func (sseScanner *SSEScanner) Next() (string, error) {
	var dataLines []string

	for sseScanner.scanner.Scan() {
		line := sseScanner.scanner.Text()

		// Empty line signals end of an event; flush accumulated data lines
		if line == "" {
			if len(dataLines) > 0 {
				payload := strings.Join(dataLines, "\n")
				return payload, nil
			}
			continue
		}

		// Skip SSE comments
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse "data:" prefix
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)

			// Check for the [DONE] sentinel (OpenAI convention)
			if data == "[DONE]" {
				return "", io.EOF
			}

			dataLines = append(dataLines, data)
			continue
		}

		// Ignore other SSE fields (event:, id:, retry:) for now
	}

	// Check for scanner errors
	if err := sseScanner.scanner.Err(); err != nil {
		return "", fmt.Errorf("SSE scanner error: %w", err)
	}

	// If we have remaining data lines when the stream ends, return them
	if len(dataLines) > 0 {
		payload := strings.Join(dataLines, "\n")
		return payload, nil
	}

	return "", io.EOF
}
