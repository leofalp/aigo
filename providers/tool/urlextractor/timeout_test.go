package urlextractor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestExtract_SlowConnection tests timeout during connection establishment
func TestExtract_SlowConnection(t *testing.T) {
	// Use a non-routable IP address to simulate connection timeout
	input := Input{
		URL:            "http://10.255.255.1:12345",
		TimeoutSeconds: 2,
	}

	start := time.Now()
	_, err := Extract(context.Background(), input)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected connection timeout error")
	}

	// Should timeout within reasonable time
	if duration > 15*time.Second {
		t.Errorf("Connection timeout took too long: %v", duration)
	}

	t.Logf("Connection timeout occurred in %v with error: %v", duration, err)
}

// TestExtract_ContextCancellationTimeout tests that context cancellation is respected
func TestExtract_ContextCancellationTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow server that would normally take a long time
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "<html><body>Test</body></html>")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 1 second
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()

	input := Input{
		URL:                   server.URL,
		TimeoutSeconds:        30, // High timeout, but context will be cancelled
		DisableSSRFProtection: true,
	}

	start := time.Now()
	_, err := Extract(ctx, input)
	duration := time.Since(start)

	// Should cancel quickly - the key test is that it doesn't hang
	if duration > 5*time.Second {
		t.Errorf("Cancellation took too long: %v (expected ~1s)", duration)
	}

	// May or may not error depending on timing, but should complete quickly
	t.Logf("Context cancellation completed in %v with error: %v", duration, err)
}

// TestExtract_HTTPClientTimeout tests that HTTP client timeout is properly configured
func TestExtract_HTTPClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay sending response headers
		time.Sleep(15 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "<html><body>Test</body></html>")
	}))
	defer server.Close()

	input := Input{
		URL:                   server.URL,
		TimeoutSeconds:        2,
		DisableSSRFProtection: true,
	}

	start := time.Now()
	_, err := Extract(context.Background(), input)
	duration := time.Since(start)

	// The extraction should fail or complete within timeout
	// It won't necessarily error (might complete with partial results)
	// but it should NOT take 15 seconds
	if duration > 5*time.Second {
		t.Errorf("Extraction took too long: %v (expected to respect timeout)", duration)
	}

	t.Logf("Extraction completed in %v with error: %v", duration, err)
}

// TestExtract_ConcurrentTimeouts tests multiple concurrent extractions with different timeouts
func TestExtract_ConcurrentTimeouts(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "<html><body>Slow</body></html>")
	}))
	defer slowServer.Close()

	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `<html><body><a href="/page1">Page 1</a></body></html>`)
	}))
	defer fastServer.Close()

	done := make(chan bool, 2)
	var slowErr error
	var fastErr error

	// Slow request with short timeout
	go func() {
		input := Input{
			URL:                   slowServer.URL,
			TimeoutSeconds:        1,
			DisableSSRFProtection: true,
		}
		_, slowErr = Extract(context.Background(), input)
		done <- true
	}()

	// Fast request that should succeed
	go func() {
		input := Input{
			URL:                    fastServer.URL,
			TimeoutSeconds:         5,
			ForceRecursiveCrawling: true,
			DisableSSRFProtection:  true,
		}
		_, fastErr = Extract(context.Background(), input)
		done <- true
	}()

	// Wait for both to complete
	timeout := time.After(10 * time.Second)
	completedCount := 0
	for completedCount < 2 {
		select {
		case <-done:
			completedCount++
		case <-timeout:
			t.Fatal("Test timed out waiting for concurrent requests")
		}
	}

	t.Logf("Slow request error: %v", slowErr)
	t.Logf("Fast request error: %v", fastErr)
}

// TestExtract_TimeoutRespected tests that overall timeout is respected during extraction
func TestExtract_TimeoutRespected(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if strings.HasSuffix(r.URL.Path, "/robots.txt") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if strings.HasSuffix(r.URL.Path, "/sitemap.xml") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Each page takes 2 seconds
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)

		// Return links to create a crawl scenario
		_, _ = fmt.Fprintf(w, `<html><body><a href="/page%d">Next</a></body></html>`, requestCount.Load())
	}))
	defer server.Close()

	input := Input{
		URL:                    server.URL,
		TimeoutSeconds:         5, // 5 second timeout
		ForceRecursiveCrawling: true,
		MaxURLs:                20,
		DisableSSRFProtection:  true,
	}

	start := time.Now()
	output, err := Extract(context.Background(), input)
	duration := time.Since(start)

	// Should complete within timeout bounds (with some margin)
	if duration > 8*time.Second {
		t.Errorf("Extraction took too long: %v (expected ~5s timeout to be respected)", duration)
	}

	t.Logf("Extraction completed in %v, error: %v, URLs found: %d, requests made: %d",
		duration, err, len(output.URLs), requestCount.Load())
}

// TestExtract_DialTimeout tests that dial timeout prevents hanging on unreachable hosts
func TestExtract_DialTimeout(t *testing.T) {
	// Use IP that will timeout during dial
	input := Input{
		URL:            "http://192.0.2.1:81", // TEST-NET-1, reserved for documentation
		TimeoutSeconds: 3,
	}

	start := time.Now()
	output, err := Extract(context.Background(), input)
	duration := time.Since(start)

	// Should timeout reasonably fast due to DialTimeout configuration
	if duration > 15*time.Second {
		t.Errorf("Dial timeout took too long: %v", duration)
	}

	// May complete without error if SSRF protection catches it first
	t.Logf("Dial timeout test completed in %v with error: %v, URLs: %d", duration, err, len(output.URLs))
}

// TestExtract_NoHangOnSlowResponse tests that extraction doesn't hang indefinitely
// on servers that are slow but still responsive
func TestExtract_NoHangOnSlowResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/robots.txt") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if strings.HasSuffix(r.URL.Path, "/sitemap.xml") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Slow but eventually responsive server
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.ResponseWriter to support flushing")
		}

		// Send data slowly but steadily
		_, _ = fmt.Fprint(w, "<html><body><h1>Slow Server</h1>")
		for i := 0; i < 10; i++ {
			_, _ = fmt.Fprintf(w, "<p>Data chunk %d</p>", i)
			flusher.Flush()
			time.Sleep(500 * time.Millisecond)
		}
		_, _ = fmt.Fprint(w, "</body></html>")
	}))
	defer server.Close()

	input := Input{
		URL:                    server.URL,
		TimeoutSeconds:         2,
		ForceRecursiveCrawling: true,
		DisableSSRFProtection:  true,
	}

	start := time.Now()
	output, err := Extract(context.Background(), input)
	duration := time.Since(start)

	// Should respect timeout and not hang indefinitely
	if duration > 5*time.Second {
		t.Errorf("Extraction hung too long: %v (expected ~2s timeout)", duration)
	}

	t.Logf("Extraction handled slow response in %v, error: %v, URLs: %d",
		duration, err, len(output.URLs))
}
