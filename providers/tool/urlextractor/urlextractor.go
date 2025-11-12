package urlextractor

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/leofalp/aigo/providers/observability"
	"github.com/leofalp/aigo/providers/tool"
	"golang.org/x/net/html"
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

	// SSRF protection: validate URL is not targeting private networks
	if !input.DisableSSRFProtection {
		if err := validateURLSafety(baseURL); err != nil {
			return Output{}, fmt.Errorf("URL safety validation failed: %w", err)
		}
	}

	// Extract observability provider from context
	observer := observability.ObserverFromContext(ctx)

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
		observer:            observer,
		progressCallback:    input.ProgressCallback,
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
	robotsCrawlDelay    int                    // Crawl-delay from robots.txt (in milliseconds)
	observer            observability.Provider // Observability provider
	progressCallback    func(int, string)      // Progress callback for long operations
}

// extract performs the complete URL extraction process
func (e *urlExtractor) extract(ctx context.Context) (Output, error) {
	// Start tracing if observability is available
	var span observability.Span
	if e.observer != nil {
		ctx, span = e.observer.StartSpan(ctx, "urlextractor.extract",
			observability.String("url", e.baseURL.String()),
			observability.Int("max_urls", e.maxURLs),
		)
		defer span.End()
	}

	output := Output{
		Sources: make(map[string]int),
	}

	// Report initial progress
	e.reportProgress(0, "initializing")

	// Step 1: Follow redirects to get the canonical base URL
	e.log(ctx, "info", "Following redirects to canonical URL", observability.String("url", e.baseURL.String()))
	canonicalURL, err := e.followRedirectsToCanonicalURL(ctx)
	if err != nil {
		e.log(ctx, "warn", "Failed to follow redirects, using original URL",
			observability.String("url", e.baseURL.String()),
			observability.Error(err),
		)
	}
	if canonicalURL != nil {
		e.baseURL = canonicalURL
		e.log(ctx, "info", "Canonical URL determined", observability.String("canonical_url", canonicalURL.String()))
	} else if err == nil {
		e.log(ctx, "debug", "No redirect found, using original URL")
	}
	output.BaseURL = e.baseURL.String()

	// Step 2: Analyze robots.txt
	e.reportProgress(0, "analyzing_robots_txt")
	e.log(ctx, "info", "Analyzing robots.txt")
	robotsFound := e.analyzeRobots(ctx)
	if robotsFound {
		output.RobotsTxtFound = true
		e.log(ctx, "info", "robots.txt found",
			observability.Int("disallowed_paths", len(e.disallowed)),
			observability.Int("sitemaps", len(e.sitemaps)),
			observability.Int("crawl_delay_ms", e.robotsCrawlDelay),
		)
	}

	// Step 3: Extract from sitemaps
	e.reportProgress(len(e.urls), "extracting_sitemaps")
	e.log(ctx, "info", "Extracting URLs from sitemaps")
	sitemapURLCount := e.extractFromSitemaps(ctx)
	if sitemapURLCount > 0 {
		output.SitemapFound = true
		output.Sources["sitemap"] = sitemapURLCount
		e.log(ctx, "info", "Sitemap extraction complete", observability.Int("urls_found", sitemapURLCount))
	}

	// Step 4: Perform crawling if forced or if no sitemaps were found at all
	shouldCrawl := e.forceRecursiveCrawl || (!output.SitemapFound && len(e.urls) == 0)
	if shouldCrawl {
		e.log(ctx, "info", "Starting recursive crawling",
			observability.Bool("forced", e.forceRecursiveCrawl),
			observability.Bool("fallback", !output.SitemapFound),
		)
		e.reportProgress(len(e.urls), "crawling")
		crawledURLCount := e.crawlWithQueue(ctx)
		if crawledURLCount > 0 {
			output.Sources["crawl"] = crawledURLCount
			e.log(ctx, "info", "Crawling complete", observability.Int("urls_found", crawledURLCount))
		}
	}

	// Convert map to slice
	output.URLs = make([]string, 0, len(e.urls))
	for urlString := range e.urls {
		output.URLs = append(output.URLs, urlString)
	}

	output.TotalURLs = len(output.URLs)

	// Report final progress
	e.reportProgress(output.TotalURLs, "complete")

	e.log(ctx, "info", "URL extraction complete",
		observability.Int("total_urls", output.TotalURLs),
		observability.Bool("robots_found", output.RobotsTxtFound),
		observability.Bool("sitemap_found", output.SitemapFound),
	)

	// Record metrics
	e.recordMetrics(ctx, output)

	return output, nil
}

// followRedirectsToCanonicalURL makes a HEAD request to the base URL and follows redirects
// to determine the canonical domain. Returns the final URL after redirects and any error encountered.
// This ensures that if example.com redirects to www.example.com, we use www.example.com as the base.
func (e *urlExtractor) followRedirectsToCanonicalURL(ctx context.Context) (*url.URL, error) {
	// Try the homepage first
	homepageURL := e.baseURL.String()

	req, err := http.NewRequestWithContext(ctx, "HEAD", homepageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HEAD request: %w", err)
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
		return resp.Request.URL, nil
	}

	return nil, nil
}

// tryRobotsTxtForCanonicalURL tries to get the canonical URL from robots.txt request
func (e *urlExtractor) tryRobotsTxtForCanonicalURL(ctx context.Context) (*url.URL, error) {
	robotsURL := e.baseURL.Scheme + "://" + e.baseURL.Host + "/robots.txt"

	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create robots.txt request: %w", err)
	}
	req.Header.Set("User-Agent", e.userAgent)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch robots.txt: %w", err)
	}
	defer resp.Body.Close()

	// Return the final URL after following redirects
	if resp.Request != nil && resp.Request.URL != nil {
		// Extract just the scheme and host, keep original path structure
		finalURL := &url.URL{
			Scheme: resp.Request.URL.Scheme,
			Host:   resp.Request.URL.Host,
		}
		return finalURL, nil
	}

	return nil, fmt.Errorf("no redirect information available")
}

// analyzeRobots fetches and parses robots.txt with proper User-Agent handling
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

	// Track which User-Agent block we're in
	currentUserAgent := ""
	inOurBlock := false // true if we're in a block for our user-agent or *

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		lineLower := strings.ToLower(line)

		// Check for User-agent directive
		if strings.HasPrefix(lineLower, "user-agent:") {
			userAgentValue := strings.TrimSpace(line[11:])
			currentUserAgent = strings.ToLower(userAgentValue)
			// We apply rules for "*" or if the user-agent matches ours
			inOurBlock = (currentUserAgent == "*" || strings.Contains(strings.ToLower(e.userAgent), currentUserAgent))
			continue
		}

		// Only process directives if we're in a relevant User-Agent block
		if !inOurBlock {
			continue
		}

		// Extract sitemap references (global, not user-agent specific)
		if strings.HasPrefix(lineLower, "sitemap:") {
			sitemapURL := strings.TrimSpace(line[8:])
			normalizedSitemapURL := e.normalizeSitemapURL(sitemapURL)
			e.sitemaps = append(e.sitemaps, normalizedSitemapURL)
			continue
		}

		// Extract disallow rules for our user-agent
		if strings.HasPrefix(lineLower, "disallow:") {
			disallowPath := strings.TrimSpace(line[9:])
			if disallowPath != "" {
				e.disallowed[disallowPath] = true
			}
			continue
		}

		// Extract crawl-delay for our user-agent
		if strings.HasPrefix(lineLower, "crawl-delay:") {
			delayStr := strings.TrimSpace(line[12:])
			if delaySeconds, err := time.ParseDuration(delayStr + "s"); err == nil {
				e.robotsCrawlDelay = int(delaySeconds.Milliseconds())
			}
			continue
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
		// Check context cancellation
		select {
		case <-ctx.Done():
			return urlCount
		default:
		}

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

		// Report progress periodically
		e.reportProgress(len(e.urls), "extracting_sitemaps")

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

		// Check if we've reached the limit before adding
		if len(e.urls) >= e.maxURLs {
			break
		}

		// Mark as visited
		e.urls[currentURL] = true

		// Report progress every 5% of maxURLs (= maxURLs/20)
		// This gives ~20 progress updates regardless of total URL count:
		// - MaxURLs=20    → step=1  (20 updates)
		// - MaxURLs=100   → step=5  (20 updates)
		// - MaxURLs=10000 → step=500 (20 updates)
		// Ensures consistent progress feedback while avoiding excessive callbacks
		step := e.maxURLs / 20
		if step < 1 {
			step = 1 // Minimum step of 1 for small extractions
		}
		if len(e.urls) > 0 && len(e.urls)%step == 0 {
			e.reportProgress(len(e.urls), "crawling")
		}

		// Apply crawl delay - use robots.txt value if higher than configured
		effectiveDelay := e.crawlDelayMs
		if e.robotsCrawlDelay > effectiveDelay {
			effectiveDelay = e.robotsCrawlDelay
		}
		if effectiveDelay > 0 {
			time.Sleep(time.Duration(effectiveDelay) * time.Millisecond)
		}

		// Fetch page and extract links
		discoveredLinks := e.extractLinks(ctx, currentURL)

		// Add new links to queue (only if we haven't reached the limit)
		for _, discoveredLink := range discoveredLinks {
			// Stop if we've reached max URLs
			if len(e.urls) >= e.maxURLs {
				break
			}

			// Only queue if not already queued and not already visited
			if !queued[discoveredLink] && !e.urls[discoveredLink] {
				if e.isSameDomain(discoveredLink) && !e.isDisallowed(discoveredLink) {
					toVisit = append(toVisit, discoveredLink)
					queued[discoveredLink] = true
				}
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

	// Limit body size BEFORE decompression to prevent memory exhaustion
	// A 5MB .gz file could decompress to 200MB without this limit
	limitedBody := io.LimitReader(resp.Body, MaxBodySize)

	// Handle gzip compression
	var reader io.Reader = limitedBody
	if strings.HasSuffix(sitemapURL, ".gz") {
		gzReader, err := gzip.NewReader(limitedBody)
		if err != nil {
			return nil, nil
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Read the (potentially decompressed) content
	body, err := io.ReadAll(reader)
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

// parseHTMLLinks extracts links from HTML using golang.org/x/net/html parser
func (e *urlExtractor) parseHTMLLinks(htmlContent, baseURL string) []string {
	discoveredLinks := make([]string, 0)

	// Parse HTML
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		// If parsing fails, return empty list (malformed HTML)
		return discoveredLinks
	}

	// Track base href if present
	baseHref := baseURL

	// Traverse DOM and extract links
	var extractLinks func(*html.Node)
	extractLinks = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "base":
				// Handle <base href="..."> tag
				for _, attr := range n.Attr {
					if attr.Key == "href" && attr.Val != "" {
						// Update base href for resolving relative URLs
						if resolved := e.resolveURL(attr.Val, baseURL); resolved != "" {
							baseHref = resolved
						}
					}
				}

			case "a", "link", "area":
				// Extract href from <a>, <link>, <area> tags
				for _, attr := range n.Attr {
					if attr.Key == "href" && attr.Val != "" {
						href := strings.TrimSpace(attr.Val)
						if e.isValidLink(href) {
							if absoluteURL := e.resolveURL(href, baseHref); absoluteURL != "" {
								discoveredLinks = append(discoveredLinks, absoluteURL)
							}
						}
					}
				}
			}
		}

		// Recursively process child nodes
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			extractLinks(child)
		}
	}

	extractLinks(doc)
	return discoveredLinks
}

// isValidLink checks if a link should be extracted
func (e *urlExtractor) isValidLink(href string) bool {
	// Skip empty links
	if href == "" {
		return false
	}

	// Skip fragments
	if strings.HasPrefix(href, "#") {
		return false
	}

	// Skip javascript: and mailto: links
	if strings.HasPrefix(strings.ToLower(href), "javascript:") ||
		strings.HasPrefix(strings.ToLower(href), "mailto:") ||
		strings.HasPrefix(strings.ToLower(href), "tel:") ||
		strings.HasPrefix(strings.ToLower(href), "data:") {
		return false
	}

	return true
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

// log is a helper function to log messages if observability is available
func (e *urlExtractor) log(ctx context.Context, level string, msg string, attrs ...observability.Attribute) {
	if e.observer == nil {
		return
	}

	switch level {
	case "debug":
		e.observer.Debug(ctx, msg, attrs...)
	case "info":
		e.observer.Info(ctx, msg, attrs...)
	case "warn":
		e.observer.Warn(ctx, msg, attrs...)
	case "error":
		e.observer.Error(ctx, msg, attrs...)
	}
}

// recordMetrics records metrics if observability is available
func (e *urlExtractor) recordMetrics(ctx context.Context, output Output) {
	if e.observer == nil {
		return
	}

	// Record total URLs extracted
	counter := e.observer.Counter("urlextractor.urls.extracted")
	counter.Add(ctx, int64(output.TotalURLs),
		observability.String("base_url", output.BaseURL),
	)

	// Record URLs by source
	for source, count := range output.Sources {
		sourceCounter := e.observer.Counter("urlextractor.urls.by_source")
		sourceCounter.Add(ctx, int64(count),
			observability.String("source", source),
			observability.String("base_url", output.BaseURL),
		)
	}
}

// validateURLSafety checks if a URL is safe to access (SSRF protection)
// It blocks access to:
// - Private IP ranges (RFC 1918: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
// - Loopback addresses (127.0.0.0/8, ::1)
// - Link-local addresses (169.254.0.0/16, fe80::/10)
// - Localhost
func validateURLSafety(u *url.URL) error {
	host := u.Hostname()

	// Check for localhost
	if host == "localhost" {
		return fmt.Errorf("localhost is not allowed")
	}

	// Try to resolve hostname to IP
	ips, err := net.LookupIP(host)
	if err != nil {
		// If we can't resolve, allow it - DNS might be temporarily unavailable
		// The actual HTTP request will fail anyway if the host doesn't exist
		return nil
	}

	// Check each resolved IP
	for _, ip := range ips {
		if isPrivateOrLocalIP(ip) {
			return fmt.Errorf("private or local IP addresses are not allowed: %s", ip.String())
		}
	}

	return nil
}

// isPrivateOrLocalIP checks if an IP is private, loopback, or link-local
func isPrivateOrLocalIP(ip net.IP) bool {
	// Loopback addresses
	if ip.IsLoopback() {
		return true
	}

	// Link-local addresses
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Private IPv4 ranges (RFC 1918)
	privateIPv4Ranges := []string{
		"10.0.0.0/8",     // 10.0.0.0 - 10.255.255.255
		"172.16.0.0/12",  // 172.16.0.0 - 172.31.255.255
		"192.168.0.0/16", // 192.168.0.0 - 192.168.255.255
	}

	for _, cidr := range privateIPv4Ranges {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}

	// Private IPv6 ranges
	if ip.To4() == nil { // IPv6
		// Unique local addresses (fc00::/7)
		_, ula, _ := net.ParseCIDR("fc00::/7")
		if ula.Contains(ip) {
			return true
		}
	}

	return false
}

// reportProgress calls the progress callback if it's set
func (e *urlExtractor) reportProgress(currentURLs int, phase string) {
	if e.progressCallback != nil {
		e.progressCallback(currentURLs, phase)
	}
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

	// ProgressCallback is called periodically during extraction to report progress
	// Parameters: currentURLs (number of URLs found so far), phase (current operation)
	// This is useful for long-running extractions to provide user feedback
	ProgressCallback func(currentURLs int, phase string) `json:"-"`

	// DisableSSRFProtection disables SSRF protection (for testing only - DO NOT USE IN PRODUCTION)
	// This allows accessing localhost and private IP ranges which are normally blocked
	DisableSSRFProtection bool `json:"-"`
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
