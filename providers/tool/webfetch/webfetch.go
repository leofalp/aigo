package webfetch

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/tool"
)

const (
	// DefaultTimeout is the default HTTP request timeout
	DefaultTimeout = 30 * time.Second
	// DefaultUserAgent is the default User-Agent header value
	DefaultUserAgent = "aigo-webfetch-tool/1.0"
	// MaxBodySize is the maximum response body size (10MB)
	MaxBodySize = 10 * 1024 * 1024
	// DialTimeout is the maximum time to wait for a TCP connection
	DialTimeout = 10 * time.Second
	// TLSHandshakeTimeout is the maximum time to wait for TLS handshake
	TLSHandshakeTimeout = 10 * time.Second
	// ResponseHeaderTimeout is the maximum time to wait for response headers
	ResponseHeaderTimeout = 10 * time.Second
	// IdleConnTimeout is the maximum time an idle connection can be reused
	IdleConnTimeout = 90 * time.Second
)

// NewWebFetchTool creates a new web fetch tool that retrieves web pages and converts HTML to Markdown.
// The tool uses the standard library's HTTP client for fetching and html-to-markdown for conversion.
//
// Features:
//   - Fetches web pages via HTTP/HTTPS
//   - Automatically adds https:// to partial URLs (e.g., "google.it" → "https://google.it")
//   - Follows HTTP redirects and returns the final URL
//   - Converts HTML content to clean Markdown
//   - Configurable timeout (default: 30s)
//   - Custom User-Agent support
//   - Response size limiting (max 10MB)
//   - Context cancellation support
//
// Example:
//
//	tool := webfetch.NewWebFetchTool()
//	client, _ := client.New(provider, client.WithTools(tool))
func NewWebFetchTool() *tool.Tool[Input, Output] {
	return tool.NewTool[Input, Output](
		"WebFetch",
		Fetch,
		tool.WithDescription("Fetches a web page and converts its HTML content to Markdown format. Supports HTTP and HTTPS protocols. Automatically handles partial URLs by adding https:// prefix. Follows redirects and returns the final URL and clean Markdown content."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.0, // Free - local HTTP client
			Currency:                "USD",
			CostDescription:         "local HTTP request",
			Accuracy:                0.98, // Very high accuracy in fetching HTML content
			AverageDurationInMillis: 350,  // Average HTTP request time (~350ms)
		}),
	)
}

// Fetch retrieves a web page from the specified URL and converts its HTML content to Markdown.
// It handles HTTP redirects, validates response status codes, and enforces size limits.
//
// The function automatically adds "https://" to partial URLs (e.g., "google.it" becomes "https://google.it").
// It follows HTTP redirects (up to 10) and returns the final URL after all redirects.
//
// The function performs the following steps:
//  1. Validates and normalizes the input URL (adds https:// if missing)
//  2. Creates an HTTP request with context for cancellation support
//  3. Fetches the page content with a timeout, following redirects
//  4. Validates the HTTP response status
//  5. Reads and limits the response body size
//  6. Converts HTML to Markdown
//
// Parameters:
//   - ctx: Context for request cancellation and timeout control
//   - req: Input containing the URL to fetch and optional configuration
//
// Returns:
//   - Output: Contains the final URL (after redirects) and converted Markdown content
//   - error: Returns error if the request fails, status is not OK, or conversion fails
//
// Example:
//
//	input := webfetch.Input{URL: "google.com"}
//	output, err := webfetch.Fetch(ctx, input)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(output.URL)      // https://www.google.com (after redirect)
//	fmt.Println(output.Markdown)
func Fetch(ctx context.Context, req Input) (Output, error) {
	// Validate and normalize URL
	url := strings.TrimSpace(req.URL)
	if url == "" {
		return Output{}, fmt.Errorf("URL cannot be empty")
	}

	// Add https:// prefix if missing
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	// Determine timeout
	timeout := DefaultTimeout
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	// Create context with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctxWithTimeout, "GET", url, nil)
	if err != nil {
		return Output{}, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent
	userAgent := DefaultUserAgent
	if req.UserAgent != "" {
		userAgent = req.UserAgent
	}
	httpReq.Header.Set("User-Agent", userAgent)

	// Create HTTP client with comprehensive timeout configuration
	// This prevents indefinite blocking on slow or unresponsive servers
	client := &http.Client{
		Timeout: timeout, // Overall request timeout
		Transport: &http.Transport{
			// DialContext controls the timeout for establishing TCP connections
			DialContext: (&net.Dialer{
				Timeout:   DialTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			// TLSHandshakeTimeout controls the timeout for TLS handshake
			TLSHandshakeTimeout: TLSHandshakeTimeout,
			// ResponseHeaderTimeout controls the timeout waiting for response headers
			ResponseHeaderTimeout: ResponseHeaderTimeout,
			// IdleConnTimeout controls how long idle connections are kept
			IdleConnTimeout: IdleConnTimeout,
			// MaxIdleConns limits the number of idle connections
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			// Disable compression to avoid issues with slow decompression
			DisableCompression: false,
			// Force attempt HTTP/2
			ForceAttemptHTTP2: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects (>10)")
			}
			return nil
		},
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		// Check if error was due to context cancellation or timeout
		if ctxWithTimeout.Err() != nil {
			return Output{}, fmt.Errorf("request timeout or canceled: %w", err)
		}
		return Output{}, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer utils.CloseWithLog(resp.Body)

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return Output{}, fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, resp.Status)
	}

	// Read body with size limit and context check
	// Use a channel to handle reading with timeout awareness
	limitedReader := io.LimitReader(resp.Body, MaxBodySize)

	type readResult struct {
		data []byte
		err  error
	}

	readChan := make(chan readResult, 1)
	go func() {
		data, err := io.ReadAll(limitedReader)
		readChan <- readResult{data: data, err: err}
	}()

	var htmlBytes []byte
	select {
	case <-ctxWithTimeout.Done():
		return Output{}, fmt.Errorf("timeout while reading response body: %w", ctxWithTimeout.Err())
	case result := <-readChan:
		if result.err != nil {
			return Output{}, fmt.Errorf("failed to read response body: %w", result.err)
		}
		htmlBytes = result.data
	}

	// Check if we hit the size limit
	if len(htmlBytes) == MaxBodySize {
		return Output{}, fmt.Errorf("response body exceeds maximum size of %d bytes", MaxBodySize)
	}

	// Convert HTML to Markdown
	markdown, err := htmltomarkdown.ConvertString(string(htmlBytes))
	if err != nil {
		return Output{}, fmt.Errorf("failed to convert HTML to Markdown: %w", err)
	}

	// Get the final URL after redirects
	finalURL := resp.Request.URL.String()

	return Output{
		URL:      finalURL,
		Markdown: markdown,
	}, nil
}

// Input represents the input parameters for the web fetch tool.
type Input struct {
	// URL is the web page URL to fetch (can be partial like "google.com" or full like "https://google.com")
	URL string `json:"url" jsonschema:"description=The URL of the web page to fetch (supports partial URLs like 'google.com' or full URLs like 'https://google.com'),required"`

	// TimeoutSeconds is the request timeout in seconds (default: 30, max: 300)
	TimeoutSeconds int `json:"timeout_seconds,omitempty" jsonschema:"description=Request timeout in seconds (default: 30 max: 300),minimum=1,maximum=300"`

	// UserAgent is the User-Agent header to send with the request (optional)
	UserAgent string `json:"user_agent,omitempty" jsonschema:"description=Custom User-Agent header for the HTTP request"`
}

// Output represents the output of the web fetch tool.
type Output struct {
	// URL is the final URL after following all redirects
	URL string `json:"url" jsonschema:"description=The final URL after following all redirects and normalization"`

	// Markdown is the page content converted from HTML to Markdown format
	Markdown string `json:"markdown" jsonschema:"description=The web page content converted to Markdown format"`
}
