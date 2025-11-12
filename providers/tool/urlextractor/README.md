# URLExtractor Tool

A comprehensive tool for extracting all URLs from a website by analyzing robots.txt, sitemap.xml files, and performing recursive crawling.

## Features

- **URL Normalization** - Automatically handles partial URLs (e.g., `example.com` → `https://example.com`)
- **Redirect Following** - Follows HTTP redirects to determine the canonical domain (e.g., `neosperience.com` → `www.neosperience.com`)
- **robots.txt Analysis** - Parses robots.txt for sitemap references and disallowed paths
- **Sitemap Support** - Extracts URLs from sitemap.xml files (including sitemap indexes)
- **Compressed Sitemaps** - Handles .gz compressed sitemap files
- **Recursive Crawling** - Falls back to crawling HTML pages if sitemaps are unavailable
- **Same-Domain Filtering** - Only extracts URLs from the same domain (handles www subdomain correctly)
- **Disallow Rules** - Respects robots.txt disallow directives
- **URL Deduplication** - Returns a unique list of URLs
- **Configurable Limits** - Control max URLs, timeout, and crawl delay
- **Polite Crawling** - Configurable delay between requests to avoid overwhelming servers
- **Pure Go Implementation** - Uses only native Go HTML parsing (no external HTML libraries)

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

## How It Works

The tool follows a multi-step process to extract URLs:

### 1. URL Normalization and Redirect Following
- Adds `https://` prefix to partial URLs
- Validates URL format
- **Follows HTTP redirects to determine the canonical domain**
  - Example: Input `neosperience.com` → redirects to `www.neosperience.com`
  - The base URL is updated to `www.neosperience.com` (the canonical domain)
  - All subsequent operations use the canonical domain
- Makes a HEAD request to the homepage to detect redirects
- Falls back to robots.txt request if homepage fails

### 2. robots.txt Analysis
- Fetches `/robots.txt` from the website
- Extracts sitemap references (e.g., `Sitemap: https://example.com/sitemap.xml`)
- Normalizes sitemap URLs to match base domain (handles www subdomain mismatches)
- Identifies disallowed paths (e.g., `Disallow: /admin/`)

### 3. Sitemap Extraction
- Checks for sitemap references from robots.txt
- Falls back to `/sitemap.xml` if not specified
- Supports sitemap index files (sitemaps containing other sitemaps)
- Handles compressed `.gz` sitemaps
- Recursively processes all discovered sitemaps

### 4. Queue-Based Crawling (When Needed or Forced)
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

### 5. Deduplication
- Removes duplicate URLs
- Returns unique list

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