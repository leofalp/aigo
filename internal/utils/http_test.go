package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---- DoPostSync tests -------------------------------------------------------

// TestDoPostSync_Success verifies that a 200 response with valid JSON is
// unmarshaled into the output struct and returned without error.
func TestDoPostSync_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"value":42}`)
	}))
	defer server.Close()

	type response struct {
		Value int `json:"value"`
	}

	_, result, err := DoPostSync[response](
		context.Background(),
		server.Client(),
		server.URL,
		"test-key",
		map[string]string{"q": "test"},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result, got nil")
	}
	if result.Value != 42 {
		t.Errorf("expected Value=42, got %d", result.Value)
	}
}

// TestDoPostSync_Non2xxStatus verifies that a non-2xx HTTP status causes
// DoPostSync to return an error that includes the status code.
func TestDoPostSync_Non2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "bad request")
	}))
	defer server.Close()

	type response struct {
		Value int `json:"value"`
	}

	_, _, err := DoPostSync[response](
		context.Background(),
		server.Client(),
		server.URL,
		"",
		map[string]string{},
	)
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected error to contain status code 400, got: %v", err)
	}
}

// TestDoPostSync_UnmarshalError verifies that a 200 response with a body that
// cannot be unmarshaled into the output struct returns an error mentioning
// "unmarshal".
func TestDoPostSync_UnmarshalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return a raw string that is not valid JSON for a struct target.
		fmt.Fprint(w, `"not json"`)
	}))
	defer server.Close()

	type response struct {
		Value int `json:"value"`
	}

	_, _, err := DoPostSync[response](
		context.Background(),
		server.Client(),
		server.URL,
		"",
		map[string]string{},
	)
	if err == nil {
		t.Fatal("expected unmarshal error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unmarshal") {
		t.Errorf("expected error to contain 'unmarshal', got: %v", err)
	}
}

// TestDoPostSync_RequestCreateError verifies that an invalid URL causes
// http.NewRequestWithContext to fail and the error is propagated.
func TestDoPostSync_RequestCreateError(t *testing.T) {
	type response struct {
		Value int `json:"value"`
	}

	// A URL with a leading space triggers a parse error in net/http.
	_, _, err := DoPostSync[response](
		context.Background(),
		nil,
		" bad url",
		"",
		map[string]string{},
	)
	if err == nil {
		t.Fatal("expected request creation error, got nil")
	}
}

// TestDoPostSync_CustomHeaders verifies that custom headers passed via
// HeaderOption are sent on the outgoing request.
func TestDoPostSync_CustomHeaders(t *testing.T) {
	const customHeaderKey = "X-Custom-Header"
	const customHeaderValue = "custom-value-123"
	var capturedHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get(customHeaderKey)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	type response struct {
		OK bool `json:"ok"`
	}

	_, _, err := DoPostSync[response](
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
	if capturedHeader != customHeaderValue {
		t.Errorf("expected custom header %q, got %q", customHeaderValue, capturedHeader)
	}
}

// TestDoPostSync_APIKeyInAuthHeader verifies that the API key is set as a
// Bearer token in the Authorization header.
func TestDoPostSync_APIKeyInAuthHeader(t *testing.T) {
	const apiKey = "mykey"
	var capturedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	type response struct {
		OK bool `json:"ok"`
	}

	_, _, err := DoPostSync[response](
		context.Background(),
		server.Client(),
		server.URL,
		apiKey,
		map[string]string{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Bearer mykey"
	if capturedAuth != expected {
		t.Errorf("expected Authorization header %q, got %q", expected, capturedAuth)
	}
}

// TestDoPostSync_NilClient_UsesDefault verifies that passing nil as the HTTP
// client causes DoPostSync to fall back to http.DefaultClient and still
// complete the request successfully.
func TestDoPostSync_NilClient_UsesDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"received":true}`)
	}))
	defer server.Close()

	type response struct {
		Received bool `json:"received"`
	}

	// Pass nil client — DoPostSync should use http.DefaultClient.
	_, result, err := DoPostSync[response](
		context.Background(),
		nil,
		server.URL,
		"",
		map[string]string{},
	)
	if err != nil {
		t.Fatalf("expected no error with nil client, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result, got nil")
	}
	if !result.Received {
		t.Error("expected Received=true, got false")
	}
}

// ---- CloseWithLog tests -----------------------------------------------------

// errCloser is a mock io.Closer that always returns the configured error.
type errCloser struct {
	closeErr error
}

func (ec *errCloser) Close() error {
	return ec.closeErr
}

// TestCloseWithLog_ErrorPath verifies that CloseWithLog does not panic when
// the underlying closer returns an error. The error is only logged via slog.
func TestCloseWithLog_ErrorPath(t *testing.T) {
	closer := &errCloser{closeErr: errors.New("close error")}

	// CloseWithLog should not panic — it only logs the error via slog.Warn.
	CloseWithLog(closer)
}
