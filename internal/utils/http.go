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

func DoPostSync[OutputStruct any](ctx context.Context, client http.Client, url string, apiKey string, body any) (*http.Response, *OutputStruct, error) {
	// Get observer from context if available
	span := observability.SpanFromContext(ctx)

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
	req.Header.Set("Authorization", "Bearer "+apiKey)

	requestStart := time.Now()
	res, err := client.Do(req)
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
	defer func(Body io.ReadCloser) {
		errS := Body.Close()
		if errS != nil { // TODO handle error into deferred function
			slog.Error("error closing response body", "error", errS.Error())
		}
	}(res.Body)

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
