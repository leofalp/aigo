//go:build integration

package webfetch

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestWebFetch_Integration verifies that the web fetch tool can retrieve and
// convert a real web page to Markdown. No API key required — this tests
// basic HTTP connectivity and HTML-to-Markdown conversion.
func TestWebFetch_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := Input{
		URL: "https://go.dev",
	}

	output, err := Fetch(ctx, input)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if output.URL == "" {
		t.Error("expected non-empty final URL")
	}

	if output.Markdown == "" {
		t.Error("expected non-empty markdown content")
	}

	t.Logf("Final URL: %s", output.URL)
	t.Logf("Markdown length: %d characters", len(output.Markdown))
}

// TestWebFetchPartialURL_Integration verifies that partial URLs (without scheme)
// are correctly normalized and fetched.
func TestWebFetchPartialURL_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := Input{
		URL: "go.dev",
	}

	output, err := Fetch(ctx, input)
	if err != nil {
		t.Fatalf("Fetch with partial URL failed: %v", err)
	}

	if output.URL == "" {
		t.Error("expected non-empty final URL")
	}

	if output.Markdown == "" {
		t.Error("expected non-empty markdown content")
	}

	t.Logf("Partial URL resolved to: %s", output.URL)
}

// TestWebFetchWithHTML_Integration verifies that raw HTML is returned when
// IncludeHTML is set to true.
func TestWebFetchWithHTML_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := Input{
		URL:         "https://go.dev",
		IncludeHTML: true,
	}

	output, err := Fetch(ctx, input)
	if err != nil {
		t.Fatalf("Fetch with IncludeHTML failed: %v", err)
	}

	if output.Markdown == "" {
		t.Error("expected non-empty markdown content")
	}

	if output.HTML == "" {
		t.Error("expected non-empty HTML content when IncludeHTML=true")
	}

	// HTML should contain basic HTML structure
	if !strings.Contains(output.HTML, "<html") && !strings.Contains(output.HTML, "<HTML") {
		t.Log("Warning: HTML content does not contain expected <html tag")
	}

	t.Logf("HTML length: %d characters", len(output.HTML))
	t.Logf("Markdown length: %d characters", len(output.Markdown))
}

// TestWebFetchRedirect_Integration verifies that HTTP redirects are followed
// correctly and the final URL is reported.
func TestWebFetchRedirect_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// golang.org redirects to go.dev
	input := Input{
		URL: "https://golang.org",
	}

	output, err := Fetch(ctx, input)
	if err != nil {
		t.Fatalf("Fetch with redirect failed: %v", err)
	}

	if output.URL == "" {
		t.Error("expected non-empty final URL after redirect")
	}

	t.Logf("Redirect: https://golang.org -> %s", output.URL)
	t.Logf("Markdown length: %d characters", len(output.Markdown))
}

// TestWebFetchCustomTimeout_Integration verifies custom timeout configuration.
func TestWebFetchCustomTimeout_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	input := Input{
		URL:            "https://go.dev",
		TimeoutSeconds: 15,
	}

	output, err := Fetch(ctx, input)
	if err != nil {
		t.Fatalf("Fetch with custom timeout failed: %v", err)
	}

	if output.Markdown == "" {
		t.Error("expected non-empty markdown content")
	}

	t.Logf("Fetch with 15s timeout succeeded, content length: %d", len(output.Markdown))
}
