package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/leofalp/aigo/providers/observability"
)

// CloseWithLog closes an io.Closer and logs any error that occurs.
// This is useful for defer statements where you want to ensure cleanup
// happens but don't want to override the main return error.
//
// Example usage:
//
//	resp, err := http.Get(url)
//	if err != nil {
//	    return err
//	}
//	defer CloseWithLog(resp.Body)
func CloseWithLog(closer io.Closer) {
	if closer == nil {
		return
	}
	if err := closer.Close(); err != nil {
		slog.Warn("failed to close resource", "error", err.Error())
	}
}

// HeaderOption represents a custom HTTP header to be added to requests.
// It holds the header name and value as strings.
type HeaderOption struct {
	Key   string
	Value string
}

// DoPostSync performs a synchronous HTTP POST request with JSON body and parses the response.
// It handles observability tracing, authorization headers, and proper resource cleanup.
//
// Error Handling Strategy:
//   - Context errors (timeout, cancellation) are propagated immediately
//   - HTTP errors (connection failures, non-2xx status) return the error
//   - Response body close errors are logged but don't override primary errors
//   - JSON parsing errors include response preview for debugging
//
// The function always closes the response body via defer, logging any close errors
// without overriding the primary error returned by the function.
//
// Custom headers can be passed via the headers variadic parameter. These headers
// can override the default Authorization header if needed (e.g., for APIs that use
// different authentication schemes like x-goog-api-key).
func DoPostSync[OutputStruct any](ctx context.Context, client *http.Client, url string, apiKey string, body any, headers ...HeaderOption) (*http.Response, *OutputStruct, error) {
	// Get observer from context if available
	span := observability.SpanFromContext(ctx)

	httpClient := client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("error marshaling body: %w", err)
	}

	if span != nil {
		span.AddEvent("http.request.prepared",
			observability.String(observability.AttrHTTPMethod, "POST"),
			observability.String(observability.AttrHTTPURL, url),
			observability.Int(observability.AttrHTTPRequestBodySize, len(jsonBody)),
		)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Apply custom headers (can override Authorization if needed)
	for _, h := range headers {
		req.Header.Set(h.Key, h.Value)
	}

	requestStart := time.Now()
	res, err := httpClient.Do(req)
	requestDuration := time.Since(requestStart)

	if err != nil {
		if span != nil {
			span.AddEvent("http.request.error",
				observability.Error(err),
				observability.Duration("http.request.duration", requestDuration),
			)
		}
		return res, nil, fmt.Errorf("error sending request: %w", err)
	}
	defer CloseWithLog(res.Body)

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading response body: %w", err)
	}

	if span != nil {
		span.AddEvent("http.response.received",
			observability.Int(observability.AttrHTTPStatusCode, res.StatusCode),
			observability.Int(observability.AttrHTTPResponseBodySize, len(respBody)),
			observability.Duration("http.request.duration", requestDuration),
		)
	}

	// Check status code
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return res, nil, fmt.Errorf("non-2xx status %d: %s", res.StatusCode, string(respBody))
	}

	var resStruct OutputStruct
	if err = json.Unmarshal(respBody, &resStruct); err != nil {
		return res, nil, fmt.Errorf("error unmarshaling LLM response body (status %d): %w\nResponse preview: %s", res.StatusCode, err, TruncateString(string(respBody), 500))
	}

	return res, &resStruct, nil
}
