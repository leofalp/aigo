# URLExtractor Tool

A comprehensive tool for extracting all URLs from a website by analyzing robots.txt, sitemap.xml files, and performing recursive crawling.

## Features

- **URL Normalization** - Automatically handles partial URLs (e.g., `example.com` → `https://example.com`)
- **Redirect Following** - Follows HTTP redirects to determine the canonical domain (e.g., `neosperience.com` → `www.neosperience.com`)
- **robots.txt Analysis** - Parses robots.txt with proper User-Agent handling for sitemap references and disallowed paths
- **Crawl-delay Support** - Respects `Crawl-delay` directive from robots.txt
- **Sitemap Support** - Extracts URLs from sitemap.xml files (including sitemap indexes)
- **Compressed Sitemaps** - Handles .gz compressed sitemap files
- **Recursive Crawling** - Falls back to crawling HTML pages if sitemaps are unavailable
- **Same-Domain Filtering** - Only extracts URLs from the same domain (handles www subdomain correctly)
- **Disallow Rules** - Respects robots.txt disallow directives (with correct User-Agent parsing)
- **URL Deduplication** - Returns a unique list of URLs
- **Configurable Limits** - Control max URLs, timeout, and crawl delay
- **Polite Crawling** - Configurable delay between requests, respects robots.txt Crawl-delay
- **SSRF Protection** - Blocks access to localhost and private IP ranges (RFC 1918) to prevent security vulnerabilities
- **Robust HTML Parsing** - Uses `golang.org/x/net/html` for standards-compliant parsing of even malformed HTML
- **Observability Integration** - Optional integration with `aigo/providers/observability` for tracing, metrics, and logging

## Usage

### Direct Use

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/leofalp/aigo/providers/tool/urlextractor"
)

func main() {
    input := urlextractor.Input{
        URL: "example.com", // Partial URLs automatically get https:// prefix
    }
    
    output, err := urlextractor.Extract(context.Background(), input)
    if err != nil {
        log.Fatalf("Failed to extract URLs: %v", err)
    }
    
    fmt.Printf("Found %d URLs\n", output.TotalURLs)
    fmt.Printf("Base URL: %s\n", output.BaseURL)
    fmt.Printf("Robots.txt found: %v\n", output.RobotsTxtFound)
    fmt.Printf("Sitemap found: %v\n", output.SitemapFound)
    
    for _, url := range output.URLs {
        fmt.Println(url)
    }
}
```

### With Custom Configuration

```go
input := urlextractor.Input{
    URL:                    "example.com",
    MaxURLs:                5000,            // Extract up to 5000 URLs
    TimeoutSeconds:         600,             // 10 minute timeout
    UserAgent:              "MyBot/1.0",
    CrawlDelayMs:           200,             // 200ms delay between requests
    ForceRecursiveCrawling: true,            // Force crawling even if sitemap exists
}

output, err := urlextractor.Extract(context.Background(), input)
```

### Integration with AI Client

```go
package main

import (
    "github.com/leofalp/aigo/core/client"
    "github.com/leofalp/aigo/providers/tool"
    "github.com/leofalp/aigo/providers/tool/urlextractor"
)

func main() {
    aiClient := client.NewClient[string](provider).
        AddTools([]tool.GenericTool{
            urlextractor.NewURLExtractorTool(),
        })
    
    response, err := aiClient.SendMessage(
        context.Background(),
        "Extract all URLs from example.com and analyze their structure",
    )
}
```

### With Observability

```go
package main

import (
    "context"
    "github.com/leofalp/aigo/providers/observability"
    "github.com/leofalp/aigo/providers/observability/slogobs"
    "github.com/leofalp/aigo/providers/tool/urlextractor"
)

func main() {
    // Create an observability provider
    observer := slogobs.New()
    
    // Add observer to context
    ctx := observability.ContextWithObserver(context.Background(), observer)
    
    input := urlextractor.Input{
        URL: "example.com",
    }
    
    // Extract will automatically use the observer from context for logging and metrics
    output, err := urlextractor.Extract(ctx, input)
    // Logs, traces, and metrics will be generated automatically
}
```

## How It Works

The tool follows a multi-step process to extract URLs:

### 1. Security Validation (SSRF Protection)
- **Validates that the target URL is not accessing private networks**
- Blocks the following to prevent Server-Side Request Forgery (SSRF):
  - Localhost (`localhost`, `127.0.0.1`, `::1`)
  - Private IPv4 ranges (RFC 1918):
    - `10.0.0.0/8` (10.0.0.0 - 10.255.255.255)
    - `172.16.0.0/12` (172.16.0.0 - 172.31.255.255)
    - `192.168.0.0/16` (192.168.0.0 - 192.168.255.255)
  - Link-local addresses (`169.254.0.0/16`, `fe80::/10`)
  - Private IPv6 ranges (`fc00::/7`)
- This protects against attacks where malicious URLs could scan internal networks

### 2. URL Normalization and Redirect Following
- Adds `https://` prefix to partial URLs
- Validates URL format
- **Follows HTTP redirects to determine the canonical domain**
  - Example: Input `neosperience.com` → redirects to `www.neosperience.com`
  - The base URL is updated to `www.neosperience.com` (the canonical domain)
  - All subsequent operations use the canonical domain
- Makes a HEAD request to the homepage to detect redirects
- Falls back to robots.txt request if homepage fails

### 3. robots.txt Analysis (with Proper User-Agent Handling)
- Fetches `/robots.txt` from the website
- **Parses User-Agent blocks correctly**:
  - Only applies rules for `User-agent: *` or matching the tool's User-Agent
  - Ignores rules meant for other bots (e.g., `User-agent: BadBot`)
  - Example:
    ```
    User-agent: BadBot
    Disallow: /          # This is ignored (not for us)
    
    User-agent: *
    Disallow: /admin/    # This is applied
    Crawl-delay: 1       # This is respected
    ```
- Extracts sitemap references (e.g., `Sitemap: https://example.com/sitemap.xml`)
- Normalizes sitemap URLs to match base domain (handles www subdomain mismatches)
- Identifies disallowed paths for the correct User-Agent
- **Respects `Crawl-delay` directive** - overrides configured delay if robots.txt specifies a longer delay

### 4. Sitemap Extraction
- Checks for sitemap references from robots.txt
- Falls back to `/sitemap.xml` if not specified
- Supports sitemap index files (sitemaps containing other sitemaps)
- Handles compressed `.gz` sitemaps
- Recursively processes all discovered sitemaps

### 5. Queue-Based Crawling (When Needed or Forced)
- **Triggered when**:
  - `ForceRecursiveCrawling` is set to true, OR
  - No sitemap found AND no URLs extracted
- **Queue-based approach** (BFS - Breadth-First Search):
  - Maintains two lists:
    - **To Visit**: Queue of URLs pending processing
    - **Visited**: URLs already processed
  - For each page:
    - Fetches and extracts all links
    - Adds new discovered links to the "To Visit" queue (if not already visited or queued)
    - Marks current page as visited
- **Behavior**:
  - If sitemap exists and contains URLs, crawling only happens when forced
  - If sitemap is missing or empty, crawling happens automatically as fallback
  - Only crawls same-domain URLs (treats `example.com` and `www.example.com` as the same domain)
  - Respects robots.txt disallow rules
  - Applies configurable delay between requests (default: 100ms) to be polite to servers

### 6. Deduplication
- Removes duplicate URLs
- Returns unique list

## Security Considerations

### SSRF Protection
This tool includes built-in protection against Server-Side Request Forgery (SSRF) attacks:

- **Blocked by default**: localhost, 127.0.0.1, private IP ranges (10.x.x.x, 192.168.x.x, 172.16-31.x.x), link-local addresses
- **Use case**: Prevents malicious actors from using the tool to scan internal networks or access internal services
- **Important**: If you're using this tool in a controlled environment where you need to access private IPs, you must explicitly acknowledge this risk

Example of blocked URLs:
```go
// These will return an error
urlextractor.Extract(ctx, urlextractor.Input{URL: "http://localhost:8080"})
urlextractor.Extract(ctx, urlextractor.Input{URL: "http://192.168.1.1"})
urlextractor.Extract(ctx, urlextractor.Input{URL: "http://10.0.0.1"})
urlextractor.Extract(ctx, urlextractor.Input{URL: "http://169.254.169.254"}) // AWS metadata
```

### robots.txt Compliance
The tool properly respects robots.txt directives:
- Parses User-Agent blocks correctly (only applies rules for `*` or matching agents)
- Respects `Disallow` directives
- Honors `Crawl-delay` directive to avoid overwhelming servers
- Higher crawl delays from robots.txt override configured delays

### Robust HTML Parsing
The tool uses `golang.org/x/net/html` for parsing HTML, which provides:
- **Standards-compliant parsing** - Handles HTML5 and older HTML versions correctly
- **Malformed HTML support** - Automatically fixes and parses broken HTML
- **Complete link extraction**:
  - `<a href="...">` - Anchor tags (standard links)
  - `<link href="...">` - Link tags (stylesheets, canonical URLs, etc.)
  - `<area href="...">` - Image map areas
  - `<base href="...">` - Base URL handling for relative links
- **Attribute format flexibility**:
  - Double quotes: `href="url"`
  - Single quotes: `href='url'`
  - No quotes: `href=url` (where valid)
  - Uppercase/mixed case: `HREF="url"` or `Href="url"`
- **Smart filtering**:
  - Skips fragments (`#`)
  - Skips JavaScript URLs (`javascript:`)
  - Skips mailto links (`mailto:`)
  - Skips telephone links (`tel:`)
  - Skips data URLs (`data:`)

Example of complex HTML that is handled correctly:
```html
<html>
<head>
  <base href="https://example.com/docs/">
  <link rel="canonical" href="https://example.com/page">
</head>
<body>
  <a href="guide.html">Relative to base</a>
  <a href="/absolute">Absolute path</a>
  <a HREF='mixed-quotes'>Mixed case</a>
  <a href="">Empty (skipped)</a>
  <a href="#">Fragment (skipped)</a>
  <a href="javascript:void(0)">JS (skipped)</a>
  
  <!-- Malformed HTML is handled gracefully -->
  <a href="page">Unclosed tag
  <div><a href="nested">Nested</a></div>
</body>
```

### Observability Integration
The tool integrates with `aigo/providers/observability` for comprehensive monitoring:

**Automatic features** (when observer is in context):
- **Distributed tracing**: Spans for each extraction phase
  - `urlextractor.extract` - Main extraction span
  - Includes attributes: URL, max_urls, configuration
- **Structured logging**: Detailed logs at each step
  - Redirect following
  - robots.txt analysis
  - Sitemap extraction
  - Crawling progress
- **Metrics collection**:
  - `urlextractor.urls.extracted` - Total URLs extracted (counter)
  - `urlextractor.urls.by_source` - URLs by source (sitemap/crawl) (counter)

**Usage**:
```go
import (
    "github.com/leofalp/aigo/providers/observability"
    "github.com/leofalp/aigo/providers/observability/slogobs"
)

// Create observer
observer := slogobs.New(
    slogobs.WithFormat(slogobs.FormatCompact),
    slogobs.WithLevel(slog.LevelInfo),
)

// Add to context
ctx := observability.ContextWithObserver(context.Background(), observer)

// Use Extract - observability is automatic
output, err := urlextractor.Extract(ctx, input)
```

**Example log output**:
```
INFO Following redirects to canonical URL url=https://example.com
INFO Canonical URL determined canonical_url=https://www.example.com
INFO Analyzing robots.txt
INFO robots.txt found disallowed_paths=2 sitemaps=1 crawl_delay_ms=1000
INFO Extracting URLs from sitemaps
INFO Sitemap extraction complete urls_found=150
INFO URL extraction complete total_urls=150 robots_found=true sitemap_found=true
```

**Benefits**:
- Debug extraction issues with detailed logs
- Monitor performance with distributed tracing
- Track metrics in production
- Correlate with other aigo components
- No performance impact when observer is nil (observability is optional)

## Input and Output

### Input

```go
// Input represents the input parameters for the URL extractor tool
type Input struct {
    // URL is the website URL to extract URLs from (required)
    // Supports both full URLs (https://example.com) and partial URLs (example.com)
    // Partial URLs automatically get https:// prefix added
    URL string `json:"url"`
    
    // MaxURLs is the maximum number of URLs to extract (default: 1000, max: 10000)
    MaxURLs int `json:"max_urls,omitempty"`
    
    // TimeoutSeconds is the total extraction timeout in seconds (default: 300, max: 600)
    TimeoutSeconds int `json:"timeout_seconds,omitempty"`
    
    // UserAgent is the User-Agent header to use (default: "aigo-urlextractor-tool/1.0")
    UserAgent string `json:"user_agent,omitempty"`
    
    // CrawlDelayMs is the delay in milliseconds between crawl requests (default: 100, max: 5000)
    CrawlDelayMs int `json:"crawl_delay_ms,omitempty"`
    
    // ForceRecursiveCrawling forces crawling even if sitemaps contain URLs (default: false)
    // If false, crawling only happens when no URLs are found from sitemaps
    ForceRecursiveCrawling bool `json:"force_recursive_crawling,omitempty"`
}
```

### Output

```go
type Output struct {
    // BaseURL is the normalized base URL that was analyzed
    BaseURL string `json:"base_url"`
    
    // URLs is the list of all extracted URLs (deduplicated)
    URLs []string `json:"urls"`
    
    // TotalURLs is the total number of unique URLs found
    TotalURLs int `json:"total_urls"`
    
    // RobotsTxtFound indicates if robots.txt was found
    RobotsTxtFound bool `json:"robots_txt_found"`
    
    // SitemapFound indicates if sitemap.xml was found
    SitemapFound bool `json:"sitemap_found"`
    
    // Sources shows how many URLs came from each source
    // Keys: "sitemap" or "crawl"
    Sources map[string]int `json:"sources"`
}
```

## Configuration Options

### MaxURLs

Limits the total number of URLs to extract.

- **Default**: 1000
- **Range**: 1-10000
- **Use case**: Prevent excessive memory usage or long extraction times

### TimeoutSeconds

Sets the maximum time for the entire extraction process.

- **Default**: 300 seconds (5 minutes)
- **Range**: 1-600 seconds (10 minutes)
- **Use case**: Prevent hanging on slow or unresponsive websites

### CrawlDelayMs

Sets the delay in milliseconds between crawl requests to be polite to the server.

- **Default**: 100 milliseconds
- **Range**: 0-5000 milliseconds
- **Use case**: Prevents overwhelming the target server with requests
- **Recommended**: 100-500ms for most sites, higher for slower servers

### ForceRecursiveCrawling

Controls whether to force crawling even when sitemaps contain URLs.

- **Default**: false
- **When false**: Crawling only happens as fallback when no sitemap found
- **When true**: Always crawl, even if sitemap contains URLs
- **Use cases**:
  - Set to `true` to discover URLs not listed in sitemap
  - Set to `true` to verify sitemap completeness
  - Leave as `false` (default) to rely on sitemap when available

**Automatic Fallback**: If no sitemap is found at all, crawling happens automatically regardless of this setting.

## Sitemap Support

### Standard Sitemap

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
    <url><loc>https://example.com/page1</loc></url>
    <url><loc>https://example.com/page2</loc></url>
</urlset>
```

### Sitemap Index

```xml
<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
    <sitemap><loc>https://example.com/sitemap1.xml</loc></sitemap>
    <sitemap><loc>https://example.com/sitemap2.xml</loc></sitemap>
</sitemapindex>
```

### Compressed Sitemaps

- Supports `.xml.gz` files
- Automatically decompresses gzip-compressed sitemaps
- No configuration needed

## robots.txt Support

The tool parses robots.txt for:

### Sitemap References

```
Sitemap: https://example.com/sitemap.xml
Sitemap: https://example.com/sitemap-products.xml
```

**Note**: If robots.txt references a sitemap with a different www subdomain (e.g., `Sitemap: https://www.example.com/sitemap.xml` when accessing `example.com`), the tool automatically normalizes the URL to match the base domain format.

### Disallow Rules

```
User-agent: *
Disallow: /admin/
Disallow: /private/
```

URLs matching disallow rules are filtered out from the results.

## HTML Crawling

When crawling HTML pages, the tool extracts links from:

- `<a href="...">` tags
- All `href` attribute variations (`href="..."`, `href='...'`, `href=...`)

The tool handles:
- **Absolute URLs**: `https://example.com/page`
- **Relative URLs**: `/page`, `page`, `./page`
- **WWW Subdomain**: Treats `example.com` and `www.example.com` as the same domain
  - Example: Input `neosperience.com`, sitemap references `www.neosperience.com` → both accepted
- **Fragments**: Ignores `#section` links
- **JavaScript**: Ignores `javascript:` URLs
- **Email**: Ignores `mailto:` links

### No External HTML Libraries

The tool uses **pure Go string parsing** for HTML link extraction, avoiding external dependencies like `goquery` or `html.Parse`. This makes it lightweight and dependency-free.

## Error Handling

The tool returns descriptive errors for various scenarios:

- **Empty URL**: `"URL cannot be empty"`
- **Invalid URL**: `"invalid URL: ..."`
- **Unsupported protocol**: `"unsupported protocol: ftp"`
- **Missing host**: `"missing host"`
- **Context timeout**: Context cancellation errors
- **Network errors**: Wrapped HTTP errors

## Limitations

- **Maximum URLs**: 10,000 URLs per extraction
- **Maximum timeout**: 600 seconds (10 minutes)
- **Maximum crawl delay**: 5000 milliseconds (5 seconds)
- **Protocols**: HTTP and HTTPS only
- **Same-domain**: Only extracts URLs from the same domain (www and non-www are considered the same)
- **Response size**: Limited to 50MB per response
- **Robots.txt**: Simplified parsing (assumes `User-agent: *`)
- **HTML parsing**: Simple pattern matching (not full HTML5 parser)

## Performance Considerations

### Memory Usage
- URLs are stored in a map for deduplication
- Large sitemaps (50MB limit) are loaded into memory
- Consider `MaxURLs` limit for sites with many pages

### Network Usage
- Each sitemap and page requires an HTTP request
- Deep crawling (`MaxDepth: 10`) can generate many requests
- Use `TimeoutSeconds` to prevent excessive network usage

### Recommended Settings

For **small websites** (< 100 pages):
```go
Input{
    MaxURLs:                500,
    CrawlDelayMs:           100,
    ForceRecursiveCrawling: true, // Force crawling to discover all pages
}
```

For **medium websites** (100-1000 pages):
```go
Input{
    MaxURLs:                2000,
    CrawlDelayMs:           200,  // Be more polite
    ForceRecursiveCrawling: false, // Only if sitemap incomplete
}
```

For **large websites** (> 1000 pages):
```go
Input{
    MaxURLs:                5000,
    CrawlDelayMs:           300,  // Be even more polite
    ForceRecursiveCrawling: false, // Rely on sitemap only
}
```

## Testing

Run tests with:

```bash
go test ./providers/tool/urlextractor/
```

For verbose output:

```bash
go test -v ./providers/tool/urlextractor/
```

The test suite includes:

- Successful URL extraction with sitemap
- Empty and invalid URL handling
- Partial URL normalization
- Sitemap index file handling
- robots.txt disallow rules
- Queue-based crawling
- Max URL limit enforcement
- Context cancellation
- Same-domain filtering
- Custom and default User-Agent
- Relative link resolution
- Fragment and special link handling
- Crawl delay functionality
- Force crawling with sitemap
- Redirect following to canonical URL

## Security Considerations

- **SSRF Risk**: Validate/sanitize user-provided URLs
- **Rate Limiting**: Consider implementing rate limiting for crawling
- **robots.txt Compliance**: The tool respects disallow rules
- **Resource Limits**: Use `MaxURLs` and `TimeoutSeconds` to prevent resource exhaustion
- **Same-Domain**: Tool only extracts same-domain URLs by design (www and non-www are treated as the same)

## Examples

### Basic Extraction

```go
input := urlextractor.Input{
    URL: "example.com", // May redirect to www.example.com
}
output, err := urlextractor.Extract(context.Background(), input)
fmt.Printf("Base URL: %s\n", output.BaseURL) // Shows canonical URL after redirects
fmt.Printf("Found %d URLs\n", len(output.URLs))
```

### Sitemap-Only Extraction (with Automatic Fallback)

```go
input := urlextractor.Input{
    URL: "example.com",
    // ForceRecursiveCrawling defaults to false
    // If sitemap exists, only sitemap will be used
    // If sitemap is missing, crawling happens automatically as fallback
}
output, err := urlextractor.Extract(context.Background(), input)
```

### Comprehensive Crawling

```go
input := urlextractor.Input{
    URL:                    "example.com",
    MaxURLs:                10000,
    CrawlDelayMs:           150,  // Polite 150ms delay
    ForceRecursiveCrawling: true, // Force crawling
}
output, err := urlextractor.Extract(context.Background(), input)
```

### With Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()

input := urlextractor.Input{
    URL: "example.com",
}
output, err := urlextractor.Extract(ctx, input)
```

### Check Source Statistics

```go
output, err := urlextractor.Extract(context.Background(), input)
if err == nil {
    fmt.Printf("URLs from sitemap: %d\n", output.Sources["sitemap"])
    fmt.Printf("URLs from crawling: %d\n", output.Sources["crawl"])
}
```

### Polite Crawling with Delay

```go
input := urlextractor.Input{
    URL:                    "example.com",
    ForceRecursiveCrawling: true,
    CrawlDelayMs:           500, // Wait 500ms between requests
    MaxURLs:                100,
}
output, err := urlextractor.Extract(context.Background(), input)
// This will take at least (100 pages * 500ms) = 50 seconds
```

## Dependencies

- `net/http` (standard library) - HTTP client
- `net/url` (standard library) - URL parsing
- `encoding/xml` (standard library) - XML parsing for sitemaps
- `compress/gzip` (standard library) - Gzip decompression
- `github.com/leofalp/aigo/providers/tool` - Tool framework

## License

This tool is part of the aigo project and follows the same license.