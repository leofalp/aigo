package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

func DoPostSync[OutputStruct any](client http.Client, url string, apiKey string, body any) (*http.Response, *OutputStruct, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("error marshaling body: %w", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	res, err := client.Do(req)
	if err != nil {
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

	// Check status code
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return res, nil, fmt.Errorf("non-2xx status %d: %s", res.StatusCode, string(respBody))
	}

	var resStruct OutputStruct
	if err = json.Unmarshal(respBody, &resStruct); err != nil {
		return res, nil, fmt.Errorf("error unmarshaling response body: %w", err)
	}

	return res, &resStruct, nil
}
