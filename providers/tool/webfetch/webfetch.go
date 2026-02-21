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

// NewWebFetchTool returns a [tool.Tool] that fetches web pages and converts
// their HTML content to Markdown. It uses the standard library HTTP client
// and the html-to-markdown library for conversion.
//
// The tool normalises partial URLs by prepending "https://", follows up to
// ten redirects, enforces a [MaxBodySize] limit, and respects context
// cancellation. The default request timeout is [DefaultTimeout].
//
// Example:
//
//	fetchTool := webfetch.NewWebFetchTool()
//	aiClient, _ := client.New(provider, client.WithTools(fetchTool))
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

// Fetch retrieves the web page at req.URL and returns its content as Markdown.
//
// Partial URLs (e.g. "google.com") are normalised by prepending "https://".
// The request timeout is taken from req.TimeoutSeconds when set, otherwise
// [DefaultTimeout] is used. Up to ten HTTP redirects are followed; the final
// URL after all redirects is returned in [Output.URL].
//
// The response body is capped at [MaxBodySize] bytes. Reading is performed in
// a goroutine so that context cancellation is honoured even during slow reads.
//
// Fetch returns an error when the URL is empty, the HTTP status code is not
// 200 OK, the body exceeds [MaxBodySize], HTML-to-Markdown conversion fails,
// or the context is cancelled or times out.
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

	output := Output{
		URL:      finalURL,
		Markdown: markdown,
	}

	// Include HTML if requested
	if req.IncludeHTML {
		output.HTML = string(htmlBytes)
	}

	return output, nil
}

// Input holds the parameters passed to the web fetch tool by the language model.
// URL is the only required field; all other fields are optional overrides for
// the defaults defined by the package-level constants.
type Input struct {
	// URL is the web page URL to fetch (can be partial like "google.com" or full like "https://google.com")
	URL string `json:"url" jsonschema:"description=The URL of the web page to fetch (supports partial URLs like 'google.com' or full URLs like 'https://google.com'),required"`

	// TimeoutSeconds is the request timeout in seconds (default: 30, max: 300)
	TimeoutSeconds int `json:"timeout_seconds,omitempty" jsonschema:"description=Request timeout in seconds (default: 30 max: 300),minimum=1,maximum=300"`

	// UserAgent is the User-Agent header to send with the request (optional)
	UserAgent string `json:"user_agent,omitempty" jsonschema:"description=Custom User-Agent header for the HTTP request"`

	// IncludeHTML when true includes the raw HTML content in the output alongside Markdown
	IncludeHTML bool `json:"include_html,omitempty" jsonschema:"description=When true includes the raw HTML content in the output (useful for logo extraction and structured data parsing)"`
}

// Output holds the result produced by [Fetch] and returned to the language model.
// URL reflects the final destination after all HTTP redirects. HTML is only
// populated when [Input.IncludeHTML] is true.
type Output struct {
	// URL is the final URL after following all redirects
	URL string `json:"url" jsonschema:"description=The final URL after following all redirects and normalization"`

	// Markdown is the page content converted from HTML to Markdown format
	Markdown string `json:"markdown" jsonschema:"description=The web page content converted to Markdown format"`

	// HTML is the raw HTML content (only populated when IncludeHTML is true in Input)
	HTML string `json:"html,omitempty" jsonschema:"description=The raw HTML content (only populated when IncludeHTML is true in Input)"`
}
