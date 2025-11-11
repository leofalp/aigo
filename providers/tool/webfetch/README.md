# WebFetch Tool

A tool for fetching web pages and converting HTML content to Markdown format.

## Features

- Fetches web pages via HTTP/HTTPS protocols
- **Automatic URL normalization** - supports partial URLs (e.g., `google.com` → `https://google.com`)
- **Follows redirects** automatically and returns the final URL (up to 10 redirects)
- Converts HTML to clean, readable Markdown using [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown)
- Configurable request timeout (default: 30 seconds)
- Custom User-Agent support
- Response size limiting (maximum 10MB)
- Context cancellation support
- Comprehensive error handling

## Usage

### Direct Use

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/leofalp/aigo/providers/tool/webfetch"
)

func main() {
    // Supports both full and partial URLs
    input := webfetch.Input{
        URL: "example.com", // Automatically becomes https://example.com
    }
    
    output, err := webfetch.Fetch(context.Background(), input)
    if err != nil {
        log.Fatalf("Failed to fetch page: %v", err)
    }
    
    fmt.Println("Final URL:", output.URL) // May differ from input due to redirects
    fmt.Println("Markdown content:")
    fmt.Println(output.Markdown)
}
```

### With Custom Options

```go
input := webfetch.Input{
    URL:            "example.com/page", // Partial URL is fine
    TimeoutSeconds: 60,
    UserAgent:      "MyBot/1.0",
}

output, err := webfetch.Fetch(context.Background(), input)
// output.URL will be the final URL after any redirects (e.g., https://www.example.com/page)
```

### Integration with AI Client

```go
package main

import (
    "github.com/leofalp/aigo/core/client"
    "github.com/leofalp/aigo/providers/tool"
    "github.com/leofalp/aigo/providers/tool/webfetch"
)

func main() {
    // Create the AI client with webfetch tool
    aiClient := client.NewClient[string](provider).
        AddTools([]tool.GenericTool{
            webfetch.NewWebFetchTool(),
        })
    
    // Now the AI can fetch and analyze web pages
    response, err := aiClient.SendMessage(
        context.Background(),
        "Fetch example.com and summarize its content", // Partial URLs work too!
    )
}
```

## Input and Output

### Input

```go
type Input struct {
    // URL is the web page URL to fetch (required)
    // Supports both full URLs (https://example.com) and partial URLs (example.com)
    // Partial URLs automatically get https:// prefix added
    URL string `json:"url"`
    
    // TimeoutSeconds is the request timeout in seconds (optional, default: 30, max: 300)
    TimeoutSeconds int `json:"timeout_seconds,omitempty"`
    
    // UserAgent is the User-Agent header to send (optional, default: "aigo-webfetch-tool/1.0")
    UserAgent string `json:"user_agent,omitempty"`
}
```

### Output

```go
type Output struct {
    // URL is the final URL after normalization and following all redirects
    // May differ from input URL (e.g., "google.com" becomes "https://www.google.com/")
    URL string `json:"url"`
    
    // Markdown is the page content converted to Markdown format
    Markdown string `json:"markdown"`
}
```

## Features in Detail

### Automatic URL Normalization

The tool automatically handles partial URLs by adding the `https://` prefix:

```go
input := webfetch.Input{URL: "google.com"}
// Becomes: https://google.com
```

```go
input := webfetch.Input{URL: "www.example.org"}
// Becomes: https://www.example.org
```

Full URLs work as expected:

```go
input := webfetch.Input{URL: "https://example.com"}
// Remains: https://example.com
```

```go
input := webfetch.Input{URL: "http://example.com"}
// Remains: http://example.com
```

**Note:** Only `http://` and `https://` protocols are supported. Other protocols (ftp://, file://, etc.) will result in fetch errors.

### Redirect Handling

The tool automatically follows HTTP redirects (301, 302, 307, 308, etc.) and returns the **final URL**:

```go
input := webfetch.Input{URL: "google.com"}
output, err := webfetch.Fetch(ctx, input)

fmt.Println(output.URL)
// Might print: https://www.google.com/
// (different from input due to redirect)
```

- Maximum 10 redirects are followed
- Exceeding the limit returns an error
- The final URL is always returned in the output

### HTML to Markdown Conversion

The tool uses the `github.com/JohannesKaufmann/html-to-markdown/v2` library to convert HTML content to clean, readable Markdown. The conversion handles:

- Headings (h1-h6)
- Paragraphs and text formatting (bold, italic, code)
- Links and images
- Lists (ordered and unordered)
- Blockquotes
- Code blocks
- Tables
- And more

### Request Timeout

Configure custom timeout to handle slow-responding servers:

```go
input := webfetch.Input{
    URL:            "https://slow-server.com",
    TimeoutSeconds: 120, // 2 minutes
}
```

Default timeout is 30 seconds. Maximum allowed is 300 seconds (5 minutes).

### Custom User-Agent

Some websites may block or limit requests from default user agents. You can specify a custom one:

```go
input := webfetch.Input{
    URL:       "https://example.com",
    UserAgent: "Mozilla/5.0 (compatible; MyBot/1.0)",
}
```

### Size Limiting

The tool automatically limits response body size to 10MB to prevent memory issues. If a response exceeds this limit, an error is returned.

### Redirect Handling

The tool automatically follows HTTP redirects (301, 302, etc.) up to a maximum of 10 redirects to prevent infinite redirect loops.

### Context Support

Full support for context cancellation and timeout:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

output, err := webfetch.Fetch(ctx, input)
```

## Error Handling

The tool returns descriptive errors for various failure scenarios:

- **Empty URL**: `"URL cannot be empty"`
- **HTTP errors**: `"unexpected status code: 404 Not Found"`
- **Timeout**: `"context deadline exceeded"`
- **Too large**: `"response body exceeds maximum size of 10485760 bytes"`
- **Too many redirects**: `"too many redirects"`
- **Network errors**: `"failed to fetch URL: ..."`
- **Conversion errors**: `"failed to convert HTML to Markdown: ..."`

**Note:** The tool now automatically normalizes partial URLs, so there's no "invalid protocol" error for URLs like "google.com".

## Limitations

- Maximum response size: 10MB
- Maximum redirects: 10
- Maximum timeout: 300 seconds (5 minutes)
- Supported protocols: HTTP and HTTPS only (automatically adds https:// to partial URLs)
- No support for authentication (basic auth, OAuth, etc.)
- No support for custom headers (except User-Agent)
- No support for POST/PUT/DELETE methods (GET only)

## Testing

Run tests with:

```bash
go test ./providers/tool/webfetch/
```

For verbose output:

```bash
go test -v ./providers/tool/webfetch/
```

The test suite includes:

- Successful page fetching and conversion
- URL validation (empty, whitespace)
- **Partial URL support** (automatic https:// prefix)
- **Redirect handling** with final URL verification
- HTTP error handling (404, 500, etc.)
- Timeout and context cancellation
- Custom User-Agent
- Redirect handling (normal and excessive)
- Large response handling
- Complex HTML structures
- Special characters and HTML entities
- Plain text responses
- Empty responses

## Dependencies

- `net/http` (standard library) - HTTP client
- `github.com/JohannesKaufmann/html-to-markdown/v2` - HTML to Markdown conversion
- `github.com/leofalp/aigo/providers/tool` - Tool framework

## Security Considerations

- The tool validates that URLs use HTTP or HTTPS protocols only
- Response size is limited to prevent memory exhaustion
- Redirect count is limited to prevent infinite loops
- User-provided URLs should be validated/sanitized before use
- Consider implementing rate limiting when fetching multiple pages
- Be aware of SSRF (Server-Side Request Forgery) risks when accepting user-provided URLs
- The tool automatically adds https:// to partial URLs - ensure this behavior is acceptable for your use case

## Performance Considerations

- Default timeout of 30 seconds balances responsiveness and reliability
- 10MB size limit protects against excessive memory usage
- HTML to Markdown conversion is performed in-memory
- Consider caching results for frequently accessed pages
- For batch operations, use goroutines with proper rate limiting

## Examples

### Basic Web Page Fetch

```go
// Works with partial URLs
input := webfetch.Input{URL: "example.com"}
output, err := webfetch.Fetch(context.Background(), input)
if err != nil {
    log.Fatal(err)
}
fmt.Println("Final URL:", output.URL) // https://example.com
fmt.Println(output.Markdown)
```

### Fetch with Timeout and Redirect Detection

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

input := webfetch.Input{URL: "google.com"} // Partial URL
output, err := webfetch.Fetch(ctx, input)
if err != nil {
    log.Fatal(err)
}

// Check if redirect happened
if output.URL != "https://google.com" {
    fmt.Println("Redirected to:", output.URL)
}
```

### Custom Configuration with Partial URL

```go
input := webfetch.Input{
    URL:            "api.example.com/docs", // Partial URL gets https:// added
    TimeoutSeconds: 60,
    UserAgent:      "MyDocBot/1.0",
}
output, err := webfetch.Fetch(context.Background(), input)
fmt.Println("Fetched from:", output.URL) // https://api.example.com/docs
```

## License

This tool is part of the aigo project and follows the same license.