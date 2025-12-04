package webfetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestFetch_SlowBodyRead tests that the fetch properly times out when the server
// sends response body data very slowly (slowloris-style attack scenario)
func TestFetch_SlowBodyRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)

		// Send data very slowly, byte by byte with delays
		// This simulates a slow or malicious server
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.ResponseWriter to support flushing")
		}

		data := []byte("<html><body><h1>Slow response</h1></body></html>")
		for i := 0; i < len(data); i++ {
			_, _ = w.Write(data[i : i+1])
			flusher.Flush()
			time.Sleep(200 * time.Millisecond) // 200ms per byte = very slow
		}
	}))
	defer server.Close()

	input := Input{
		URL:            server.URL,
		TimeoutSeconds: 2, // Should timeout before all data is sent
	}

	start := time.Now()
	_, err := Fetch(context.Background(), input)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error for slow body read")
	}

	// Verify it's a timeout error
	if !strings.Contains(err.Error(), "timeout") &&
		!strings.Contains(err.Error(), "context deadline exceeded") &&
		!strings.Contains(err.Error(), "cancelled") {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	// Verify the timeout happened within reasonable bounds (not much longer than expected)
	if duration > 4*time.Second {
		t.Errorf("Timeout took too long: %v (expected ~2s)", duration)
	}
}

// TestFetch_SlowConnection tests timeout during connection establishment
func TestFetch_SlowConnection(t *testing.T) {
	// Use a non-routable IP address to simulate connection timeout
	// 10.255.255.1 is a reserved IP that should not respond
	input := Input{
		URL:            "http://10.255.255.1:12345",
		TimeoutSeconds: 2,
	}

	start := time.Now()
	_, err := Fetch(context.Background(), input)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected connection timeout error")
	}

	// Should timeout within reasonable time
	if duration > 15*time.Second {
		t.Errorf("Connection timeout took too long: %v", duration)
	}
}

// TestFetch_SlowHeaders tests timeout while waiting for response headers
func TestFetch_SlowHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay before sending any response
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>Test</body></html>"))
	}))
	defer server.Close()

	input := Input{
		URL:            server.URL,
		TimeoutSeconds: 2,
	}

	start := time.Now()
	_, err := Fetch(context.Background(), input)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error waiting for headers")
	}

	if !strings.Contains(err.Error(), "timeout") &&
		!strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	// Should timeout around 2 seconds, not 5
	if duration > 4*time.Second {
		t.Errorf("Header timeout took too long: %v (expected ~2s)", duration)
	}
}

// TestFetch_ContextCancellationDuringRead tests that context cancellation
// works properly during body reading
func TestFetch_ContextCancellationDuringRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.ResponseWriter to support flushing")
		}

		// Send data slowly over 10 seconds
		for i := 0; i < 50; i++ {
			_, _ = w.Write([]byte("<p>Data chunk</p>"))
			flusher.Flush()
			time.Sleep(200 * time.Millisecond)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 1 second
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()

	input := Input{
		URL:            server.URL,
		TimeoutSeconds: 30, // High timeout, but context will be cancelled
	}

	start := time.Now()
	_, err := Fetch(ctx, input)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected cancellation error")
	}

	if !strings.Contains(err.Error(), "cancel") &&
		!strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected cancellation error, got: %v", err)
	}

	// Should cancel quickly, not wait for full response
	if duration > 3*time.Second {
		t.Errorf("Cancellation took too long: %v (expected ~1s)", duration)
	}
}

// TestFetch_PartialBodyTimeout tests timeout when body is only partially read
func TestFetch_PartialBodyTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Length", "1000000") // Claim large content
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.ResponseWriter to support flushing")
		}

		// Send some data quickly
		_, _ = w.Write([]byte("<html><body>"))
		flusher.Flush()

		// Then stall
		time.Sleep(10 * time.Second)
		_, _ = w.Write([]byte("</body></html>"))
	}))
	defer server.Close()

	input := Input{
		URL:            server.URL,
		TimeoutSeconds: 2,
	}

	start := time.Now()
	_, err := Fetch(context.Background(), input)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error during partial body read")
	}

	if !strings.Contains(err.Error(), "timeout") &&
		!strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	// Should timeout around 2 seconds
	if duration > 4*time.Second {
		t.Errorf("Timeout took too long: %v (expected ~2s)", duration)
	}
}

// TestFetch_ConcurrentRequests tests that multiple concurrent requests
// with different timeouts work correctly and don't interfere with each other
func TestFetch_ConcurrentRequests(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>Slow</body></html>"))
	}))
	defer slowServer.Close()

	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>Fast</body></html>"))
	}))
	defer fastServer.Close()

	done := make(chan bool, 2)

	// Slow request that should timeout
	go func() {
		input := Input{
			URL:            slowServer.URL,
			TimeoutSeconds: 1,
		}
		_, err := Fetch(context.Background(), input)
		if err == nil {
			t.Error("Expected timeout error for slow request")
		}
		done <- true
	}()

	// Fast request that should succeed
	go func() {
		input := Input{
			URL:            fastServer.URL,
			TimeoutSeconds: 5,
		}
		_, err := Fetch(context.Background(), input)
		if err != nil {
			t.Errorf("Expected success for fast request, got: %v", err)
		}
		done <- true
	}()

	// Wait for both to complete
	<-done
	<-done
}
