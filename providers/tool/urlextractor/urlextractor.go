package urlextractor

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/leofalp/aigo/providers/tool"
)

const (
	// DefaultTimeout is the default HTTP request timeout
	DefaultTimeout = 30 * time.Second
	// DefaultUserAgent is the default User-Agent header value
	DefaultUserAgent = "aigo-urlextractor-tool/1.0"
	// MaxBodySize is the maximum response body size (50MB for large sitemaps)
	MaxBodySize = 50 * 1024 * 1024
	// DefaultMaxURLs is the default maximum number of URLs to extract
	DefaultMaxURLs = 1000
	// DefaultCrawlDelayMs is the default delay in milliseconds between crawl requests
	DefaultCrawlDelayMs = 100
	// MaxCrawlTimeout is the maximum total time for crawling
	MaxCrawlTimeout = 5 * time.Minute
)

// NewURLExtractorTool creates a new URL extraction tool that extracts all URLs from a website.
// The tool analyzes robots.txt, sitemap.xml files, and performs recursive crawling if needed.
//
// Features:
//   - Normalizes URLs and follows HTTP redirects to determine canonical domain
//   - Analyzes robots.txt for sitemap references and disallowed paths
//   - Extracts URLs from sitemap.xml (including sitemap index files)
//   - Supports compressed sitemaps (.gz)
//   - Falls back to queue-based crawling if needed or forced
//   - Deduplicates URLs
//   - Respects same-domain constraint (handles www subdomain correctly)
//   - Configurable URL limits and crawl delay
//
// Example:
//
//	tool := urlextractor.NewURLExtractorTool()
//	client := client.NewClient(provider).AddTools([]tool.GenericTool{tool})
func NewURLExtractorTool() *tool.Tool[Input, Output] {
	return tool.NewTool[Input, Output](
		"URLExtractor",
		Extract,
		tool.WithDescription("Extracts all URLs from a website by analyzing robots.txt, sitemap.xml files, and performing recursive crawling if needed. Returns a deduplicated list of URLs from the same domain."),
	)
}

// Extract extracts all URLs from the specified website.
// It follows this process:
//  1. Normalizes the input URL and follows HTTP redirects to determine the canonical domain
//     (e.g., "neosperience.com" may redirect to "www.neosperience.com")
//  2. Analyzes robots.txt for sitemap references and disallowed paths
//  3. Extracts URLs from sitemap.xml files (including sitemap indexes)
//  4. If sitemaps yield no URLs or ForceRecursiveCrawling is true, performs queue-based crawling
//  5. Returns a deduplicated list of URLs
//
// The base URL in the output reflects the canonical domain after following redirects.
//
// Parameters:
//   - ctx: Context for request cancellation and timeout control
//   - input: Input containing the URL and optional configuration
//
// Returns:
//   - Output: Contains the extracted URLs, canonical base URL, statistics, and source information
//   - error: Returns error if the extraction fails
//
// Example:
//
//	input := urlextractor.Input{URL: "example.com"}
//	output, err := urlextractor.Extract(ctx, input)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Base URL: %s\n", output.BaseURL) // May be www.example.com after redirect
//	fmt.Printf("Found %d URLs\n", len(output.URLs))
func Extract(ctx context.Context, input Input) (Output, error) {
	// Validate and normalize URL
	baseURL, err := normalizeURL(input.URL)
	if err != nil {
		return Output{}, fmt.Errorf("invalid URL: %w", err)
	}

	// Create extractor
	extractor := &urlExtractor{
		baseURL:             baseURL,
		urls:                make(map[string]bool),
		disallowed:          make(map[string]bool),
		maxURLs:             DefaultMaxURLs,
		userAgent:           DefaultUserAgent,
		client:              &http.Client{Timeout: DefaultTimeout},
		forceRecursiveCrawl: input.ForceRecursiveCrawling,
		crawlDelayMs:        DefaultCrawlDelayMs,
	}

	// Apply custom configuration
	if input.MaxURLs > 0 {
		extractor.maxURLs = input.MaxURLs
	}
	if input.UserAgent != "" {
		extractor.userAgent = input.UserAgent
	}
	if input.CrawlDelayMs > 0 {
		extractor.crawlDelayMs = input.CrawlDelayMs
	}

	// Set extraction timeout
	timeout := MaxCrawlTimeout
	if input.TimeoutSeconds > 0 {
		timeout = time.Duration(input.TimeoutSeconds) * time.Second
	}

	extractCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Extract URLs
	return extractor.extract(extractCtx)
}

// urlExtractor handles the URL extraction process
type urlExtractor struct {
	baseURL             *url.URL
	urls                map[string]bool
	disallowed          map[string]bool
	sitemaps            []string
	maxURLs             int
	userAgent           string
	client              *http.Client
	forceRecursiveCrawl bool
	crawlDelayMs        int
}

// extract performs the complete URL extraction process
func (e *urlExtractor) extract(ctx context.Context) (Output, error) {
	output := Output{
		Sources: make(map[string]int),
	}

	// Step 1: Follow redirects to get the canonical base URL
	canonicalURL := e.followRedirectsToCanonicalURL(ctx)
	if canonicalURL != nil {
		e.baseURL = canonicalURL
	}
	output.BaseURL = e.baseURL.String()

	// Step 2: Analyze robots.txt
	robotsFound := e.analyzeRobots(ctx)
	if robotsFound {
		output.RobotsTxtFound = true
	}

	// Step 3: Extract from sitemaps
	sitemapURLCount := e.extractFromSitemaps(ctx)
	if sitemapURLCount > 0 {
		output.SitemapFound = true
		output.Sources["sitemap"] = sitemapURLCount
	}

	// Step 4: Perform crawling if forced or if no sitemaps were found at all
	shouldCrawl := e.forceRecursiveCrawl || (!output.SitemapFound && len(e.urls) == 0)
	if shouldCrawl {
		crawledURLCount := e.crawlWithQueue(ctx)
		if crawledURLCount > 0 {
			output.Sources["crawl"] = crawledURLCount
		}
	}

	// Convert map to slice
	output.URLs = make([]string, 0, len(e.urls))
	for urlString := range e.urls {
		output.URLs = append(output.URLs, urlString)
	}

	output.TotalURLs = len(output.URLs)

	return output, nil
}

// followRedirectsToCanonicalURL makes a HEAD request to the base URL and follows redirects
// to determine the canonical domain. Returns the final URL after redirects, or nil on error.
// This ensures that if example.com redirects to www.example.com, we use www.example.com as the base.
func (e *urlExtractor) followRedirectsToCanonicalURL(ctx context.Context) *url.URL {
	// Try the homepage first
	homepageURL := e.baseURL.String()

	req, err := http.NewRequestWithContext(ctx, "HEAD", homepageURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", e.userAgent)

	// Create a client that follows redirects
	client := &http.Client{
		Timeout: DefaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		// If HEAD fails, try GET on robots.txt
		return e.tryRobotsTxtForCanonicalURL(ctx)
	}
	defer resp.Body.Close()

	// Return the final URL after following redirects
	if resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL
	}

	return nil
}

// tryRobotsTxtForCanonicalURL tries to get the canonical URL from robots.txt request
func (e *urlExtractor) tryRobotsTxtForCanonicalURL(ctx context.Context) *url.URL {
	robotsURL := e.baseURL.Scheme + "://" + e.baseURL.Host + "/robots.txt"

	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", e.userAgent)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	// Return the final URL after following redirects
	if resp.Request != nil && resp.Request.URL != nil {
		// Extract just the scheme and host, keep original path structure
		finalURL := &url.URL{
			Scheme: resp.Request.URL.Scheme,
			Host:   resp.Request.URL.Host,
		}
		return finalURL
	}

	return nil
}

// analyzeRobots fetches and parses robots.txt
func (e *urlExtractor) analyzeRobots(ctx context.Context) bool {
	robotsURL := e.baseURL.Scheme + "://" + e.baseURL.Host + "/robots.txt"

	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", e.userAgent)

	resp, err := e.client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Extract sitemap references
		if strings.HasPrefix(strings.ToLower(line), "sitemap:") {
			sitemapURL := strings.TrimSpace(line[8:])
			// Normalize sitemap URL to handle www subdomain
			normalizedSitemapURL := e.normalizeSitemapURL(sitemapURL)
			e.sitemaps = append(e.sitemaps, normalizedSitemapURL)
		}

		// Extract disallow rules (simplified - only for * user-agent)
		if strings.HasPrefix(strings.ToLower(line), "disallow:") {
			disallowPath := strings.TrimSpace(line[9:])
			if disallowPath != "" {
				e.disallowed[disallowPath] = true
			}
		}
	}

	return true
}

// normalizeSitemapURL normalizes a sitemap URL to match the base domain
// If the sitemap URL has a www subdomain but base doesn't (or vice versa),
// it adjusts the sitemap URL to match the base URL's format.
// This is kept as a fallback for edge cases where redirects weren't followed.
func (e *urlExtractor) normalizeSitemapURL(sitemapURL string) string {
	parsedSitemap, err := url.Parse(sitemapURL)
	if err != nil {
		return sitemapURL
	}

	// Get normalized hosts (without www)
	baseHostNormalized := e.normalizeHost(e.baseURL.Host)
	sitemapHostNormalized := e.normalizeHost(parsedSitemap.Host)

	// If the normalized hosts match, use the base URL's host format
	if baseHostNormalized == sitemapHostNormalized {
		parsedSitemap.Host = e.baseURL.Host
		return parsedSitemap.String()
	}

	return sitemapURL
}

// extractFromSitemaps extracts URLs from all discovered sitemaps
func (e *urlExtractor) extractFromSitemaps(ctx context.Context) int {
	// If no sitemaps from robots.txt, try default location
	if len(e.sitemaps) == 0 {
		defaultSitemap := e.baseURL.Scheme + "://" + e.baseURL.Host + "/sitemap.xml"
		e.sitemaps = append(e.sitemaps, defaultSitemap)
	}

	urlCount := 0
	processedSitemaps := make(map[string]bool)

	for len(e.sitemaps) > 0 && len(e.urls) < e.maxURLs {
		// Pop sitemap from queue
		sitemapURL := e.sitemaps[0]
		e.sitemaps = e.sitemaps[1:]

		// Skip if already processed
		if processedSitemaps[sitemapURL] {
			continue
		}
		processedSitemaps[sitemapURL] = true

		// Fetch and parse sitemap
		extractedURLs, additionalSitemaps := e.parseSitemap(ctx, sitemapURL)
		urlCount += len(extractedURLs)

		// Add new sitemaps to queue
		for _, additionalSitemap := range additionalSitemaps {
			if !processedSitemaps[additionalSitemap] {
				e.sitemaps = append(e.sitemaps, additionalSitemap)
			}
		}

		// Add URLs
		for _, extractedURL := range extractedURLs {
			if len(e.urls) >= e.maxURLs {
				break
			}
			if e.isSameDomain(extractedURL) && !e.isDisallowed(extractedURL) {
				e.urls[extractedURL] = true
			}
		}
	}

	return urlCount
}

// crawlWithQueue performs queue-based crawling starting from the base URL
// This uses a BFS (breadth-first search) approach with a queue of URLs to visit
func (e *urlExtractor) crawlWithQueue(ctx context.Context) int {
	initialCount := len(e.urls)

	// Queue of URLs to visit (pending)
	toVisit := []string{e.baseURL.String()}
	// Track which URLs we've already queued to avoid duplicates in queue
	queued := make(map[string]bool)
	queued[e.baseURL.String()] = true

	for len(toVisit) > 0 && len(e.urls) < e.maxURLs {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return len(e.urls) - initialCount
		default:
		}

		// Pop the first URL from queue
		currentURL := toVisit[0]
		toVisit = toVisit[1:]

		// Skip if already visited
		if e.urls[currentURL] {
			continue
		}

		// Skip if not same domain or disallowed
		if !e.isSameDomain(currentURL) || e.isDisallowed(currentURL) {
			continue
		}

		// Mark as visited
		e.urls[currentURL] = true

		// Apply crawl delay if configured (be polite to the server)
		if e.crawlDelayMs > 0 {
			time.Sleep(time.Duration(e.crawlDelayMs) * time.Millisecond)
		}

		// Fetch page and extract links
		discoveredLinks := e.extractLinks(ctx, currentURL)

		// Add new links to queue
		for _, discoveredLink := range discoveredLinks {
			// Only queue if not already queued and not already visited
			if !queued[discoveredLink] && !e.urls[discoveredLink] {
				if e.isSameDomain(discoveredLink) && !e.isDisallowed(discoveredLink) {
					toVisit = append(toVisit, discoveredLink)
					queued[discoveredLink] = true
				}
			}

			// Stop adding if we've reached max URLs
			if len(e.urls)+len(toVisit) >= e.maxURLs {
				break
			}
		}
	}

	return len(e.urls) - initialCount
}

// parseSitemap fetches and parses a sitemap XML file
func (e *urlExtractor) parseSitemap(ctx context.Context, sitemapURL string) ([]string, []string) {
	req, err := http.NewRequestWithContext(ctx, "GET", sitemapURL, nil)
	if err != nil {
		return nil, nil
	}
	req.Header.Set("User-Agent", e.userAgent)

	resp, err := e.client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	defer resp.Body.Close()

	// Handle gzip compression
	var reader io.Reader = resp.Body
	if strings.HasSuffix(sitemapURL, ".gz") {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, nil
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Limit body size
	limitedReader := io.LimitReader(reader, MaxBodySize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, nil
	}

	// Try parsing as sitemap index first
	var sitemapIndex SitemapIndex
	if err := xml.Unmarshal(body, &sitemapIndex); err == nil && len(sitemapIndex.Sitemaps) > 0 {
		sitemapURLs := make([]string, 0, len(sitemapIndex.Sitemaps))
		for _, sitemapEntry := range sitemapIndex.Sitemaps {
			sitemapURLs = append(sitemapURLs, sitemapEntry.Loc)
		}
		return nil, sitemapURLs
	}

	// Parse as regular sitemap
	var sitemap Sitemap
	if err := xml.Unmarshal(body, &sitemap); err != nil {
		return nil, nil
	}

	extractedURLs := make([]string, 0, len(sitemap.URLs))
	for _, urlEntry := range sitemap.URLs {
		extractedURLs = append(extractedURLs, urlEntry.Loc)
	}

	return extractedURLs, nil
}

// extractLinks fetches a page and extracts all links
func (e *urlExtractor) extractLinks(ctx context.Context, pageURL string) []string {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", e.userAgent)

	resp, err := e.client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	// Only process HTML content
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/html") {
		return nil
	}

	// Read body with limit
	limitedReader := io.LimitReader(resp.Body, MaxBodySize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil
	}

	// Extract links using simple HTML parsing (no external dependencies)
	return e.parseHTMLLinks(string(body), pageURL)
}

// parseHTMLLinks extracts links from HTML using native Go string parsing
func (e *urlExtractor) parseHTMLLinks(htmlContent, baseURL string) []string {
	discoveredLinks := make([]string, 0)
	lowerHTML := strings.ToLower(htmlContent)

	// Find all href attributes in lowercase version, extract from original
	searchPatterns := []string{
		`href="`,
		`href='`,
	}

	for _, pattern := range searchPatterns {
		searchStartIndex := 0
		for {
			patternIndex := strings.Index(lowerHTML[searchStartIndex:], pattern)
			if patternIndex == -1 {
				break
			}
			patternIndex = searchStartIndex + patternIndex + len(pattern)

			// Find end of URL based on quote type
			endQuoteChar := pattern[len(pattern)-1:] // " or '
			endQuoteIndex := strings.Index(lowerHTML[patternIndex:], endQuoteChar)
			if endQuoteIndex == -1 {
				searchStartIndex = patternIndex
				continue
			}
			endQuoteIndex = patternIndex + endQuoteIndex

			if endQuoteIndex > patternIndex {
				// Extract from original HTML (not lowercased)
				linkHref := strings.TrimSpace(htmlContent[patternIndex:endQuoteIndex])
				if linkHref != "" && !strings.HasPrefix(linkHref, "#") && !strings.HasPrefix(linkHref, "javascript:") && !strings.HasPrefix(linkHref, "mailto:") {
					// Resolve relative URLs
					if absoluteURL := e.resolveURL(linkHref, baseURL); absoluteURL != "" {
						discoveredLinks = append(discoveredLinks, absoluteURL)
					}
				}
			}

			searchStartIndex = endQuoteIndex + 1
			if searchStartIndex >= len(lowerHTML) {
				break
			}
		}
	}

	return discoveredLinks
}

// resolveURL converts relative URLs to absolute
func (e *urlExtractor) resolveURL(relativeOrAbsoluteLink, baseURLString string) string {
	// Already absolute
	if strings.HasPrefix(relativeOrAbsoluteLink, "http://") || strings.HasPrefix(relativeOrAbsoluteLink, "https://") {
		return relativeOrAbsoluteLink
	}

	parsedBaseURL, err := url.Parse(baseURLString)
	if err != nil {
		return ""
	}

	parsedLinkURL, err := url.Parse(relativeOrAbsoluteLink)
	if err != nil {
		return ""
	}

	return parsedBaseURL.ResolveReference(parsedLinkURL).String()
}

// isSameDomain checks if a URL belongs to the same domain as the base URL
// This function handles www subdomain variations (e.g., example.com and www.example.com are considered the same)
func (e *urlExtractor) isSameDomain(checkURL string) bool {
	parsedURL, err := url.Parse(checkURL)
	if err != nil {
		return false
	}

	// Normalize both hosts by removing www. prefix if present
	baseHost := e.normalizeHost(e.baseURL.Host)
	checkHost := e.normalizeHost(parsedURL.Host)

	return baseHost == checkHost
}

// normalizeHost removes www. prefix from a hostname for comparison
func (e *urlExtractor) normalizeHost(hostname string) string {
	if strings.HasPrefix(hostname, "www.") {
		return hostname[4:]
	}
	return hostname
}

// isDisallowed checks if a URL is disallowed by robots.txt
func (e *urlExtractor) isDisallowed(checkURL string) bool {
	parsedURL, err := url.Parse(checkURL)
	if err != nil {
		return false
	}

	for disallowPath := range e.disallowed {
		if strings.HasPrefix(parsedURL.Path, disallowPath) {
			return true
		}
	}
	return false
}

// normalizeURL validates and normalizes a URL
func normalizeURL(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("URL cannot be empty")
	}

	// Check if it has a scheme already
	hasScheme := strings.Contains(rawURL, "://")

	// If it has a scheme, validate it's http or https
	if hasScheme {
		if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
			return nil, fmt.Errorf("unsupported protocol")
		}
	} else {
		// Add https:// if no scheme
		rawURL = "https://" + rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported protocol: %s", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return nil, fmt.Errorf("missing host")
	}

	return parsedURL, nil
}

// Input represents the input parameters for the URL extractor tool
type Input struct {
	// URL is the website URL to extract URLs from (can be partial like "example.com")
	URL string `json:"url" jsonschema:"description=The website URL to extract URLs from (supports partial URLs),required"`

	// MaxURLs is the maximum number of URLs to extract (default: 1000)
	MaxURLs int `json:"max_urls,omitempty" jsonschema:"description=Maximum number of URLs to extract (default: 1000),minimum=1,maximum=10000"`

	// TimeoutSeconds is the total extraction timeout in seconds (default: 300)
	TimeoutSeconds int `json:"timeout_seconds,omitempty" jsonschema:"description=Total extraction timeout in seconds (default: 300),minimum=1,maximum=600"`

	// UserAgent is the User-Agent header to use (optional)
	UserAgent string `json:"user_agent,omitempty" jsonschema:"description=Custom User-Agent header for HTTP requests"`

	// CrawlDelayMs is the delay in milliseconds between crawl requests (default: 100)
	// Use this to be polite to the server and avoid overwhelming it
	CrawlDelayMs int `json:"crawl_delay_ms,omitempty" jsonschema:"description=Delay in milliseconds between crawl requests (default: 100),minimum=0,maximum=5000"`

	// ForceRecursiveCrawling forces recursive crawling even if sitemaps are found
	// If false, crawling only happens when no URLs are found from sitemaps
	ForceRecursiveCrawling bool `json:"force_recursive_crawling,omitempty" jsonschema:"description=Force recursive crawling even if sitemaps contain URLs"`
}

// Output represents the output of the URL extractor tool
type Output struct {
	// BaseURL is the canonical base URL after following redirects
	BaseURL string `json:"base_url" jsonschema:"description=The canonical base URL after following redirects (e.g. www.example.com if example.com redirects to it)"`

	// URLs is the list of extracted URLs
	URLs []string `json:"urls" jsonschema:"description=List of all extracted URLs from the website"`

	// TotalURLs is the total number of URLs found
	TotalURLs int `json:"total_urls" jsonschema:"description=Total number of URLs extracted"`

	// RobotsTxtFound indicates if robots.txt was found
	RobotsTxtFound bool `json:"robots_txt_found" jsonschema:"description=Whether robots.txt file was found and analyzed"`

	// SitemapFound indicates if sitemap.xml was found
	SitemapFound bool `json:"sitemap_found" jsonschema:"description=Whether sitemap.xml files were found and processed"`

	// Sources shows how many URLs came from each source (sitemap, crawl)
	Sources map[string]int `json:"sources" jsonschema:"description=Number of URLs discovered from each source (sitemap or crawl)"`
}

// Sitemap represents a sitemap.xml structure
type Sitemap struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

// SitemapIndex represents a sitemap index structure
type SitemapIndex struct {
	XMLName  xml.Name `xml:"sitemapindex"`
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}
