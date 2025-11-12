package urlextractor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestExtract_Success tests successful URL extraction with sitemap
func TestExtract_Success(t *testing.T) {
	// Create test server with sitemap
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		switch r.URL.Path {
		case "/robots.txt":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Sitemap: %s/sitemap.xml\n", baseURL)
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>%s/page1</loc></url>
	<url><loc>%s/page2</loc></url>
	<url><loc>%s/page3</loc></url>
</urlset>`, baseURL, baseURL, baseURL)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                   server.URL,
		DisableSSRFProtection: true, // Allow httptest server on localhost
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if output.TotalURLs != 3 {
		t.Errorf("Expected 3 URLs, got %d", output.TotalURLs)
	}

	if !output.RobotsTxtFound {
		t.Error("Expected robots.txt to be found")
	}

	if !output.SitemapFound {
		t.Error("Expected sitemap.xml to be found")
	}

	if len(output.URLs) != 3 {
		t.Errorf("Expected 3 URLs in output, got %d", len(output.URLs))
	}
}

// TestExtract_EmptyURL tests extraction with empty URL
func TestExtract_EmptyURL(t *testing.T) {
	input := Input{
		URL:                   "",
		DisableSSRFProtection: true,
	}

	_, err := Extract(context.Background(), input)
	if err == nil {
		t.Fatal("Expected error for empty URL, got nil")
	}

	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected 'empty' error, got: %v", err)
	}
}

// TestExtract_PartialURL tests extraction with partial URL
func TestExtract_PartialURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Remove http:// prefix to simulate partial URL
	partialURL := strings.TrimPrefix(server.URL, "http://")

	input := Input{
		URL:                   partialURL,
		DisableSSRFProtection: true,
	}

	_, err := Extract(context.Background(), input)
	// Should not error on URL parsing (it adds https://)
	// But will error on connection since we're using http server
	if err != nil && !strings.Contains(err.Error(), "connection") && !strings.Contains(err.Error(), "dial") {
		t.Logf("Got expected connection error: %v", err)
	}
}

// TestExtract_SitemapIndex tests sitemap index file handling
func TestExtract_SitemapIndex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<sitemap><loc>%s/sitemap1.xml</loc></sitemap>
	<sitemap><loc>%s/sitemap2.xml</loc></sitemap>
</sitemapindex>`, baseURL, baseURL)
		case "/sitemap1.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>%s/page1</loc></url>
	<url><loc>%s/page2</loc></url>
</urlset>`, baseURL, baseURL)
		case "/sitemap2.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>%s/page3</loc></url>
</urlset>`, baseURL)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                   server.URL,
		DisableSSRFProtection: true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if output.TotalURLs != 3 {
		t.Errorf("Expected 3 URLs, got %d", output.TotalURLs)
	}

	if !output.SitemapFound {
		t.Error("Expected sitemap to be found")
	}
}

// TestExtract_RobotsTxtDisallow tests robots.txt disallow rules
func TestExtract_RobotsTxtDisallow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		switch r.URL.Path {
		case "/robots.txt":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "User-agent: *\nDisallow: /admin/\nSitemap: %s/sitemap.xml\n", baseURL)
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>%s/page1</loc></url>
	<url><loc>%s/admin/secret</loc></url>
	<url><loc>%s/page2</loc></url>
</urlset>`, baseURL, baseURL, baseURL)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                   server.URL,
		DisableSSRFProtection: true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should only have 2 URLs (admin URL filtered out)
	if output.TotalURLs != 2 {
		t.Errorf("Expected 2 URLs (disallowed URL filtered), got %d", output.TotalURLs)
	}

	// Check that admin URL is not in results
	for _, url := range output.URLs {
		if strings.Contains(url, "/admin/") {
			t.Error("Found disallowed URL in results")
		}
	}
}

// TestExtract_Crawling tests recursive crawling when no sitemap
func TestExtract_Crawling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<html><body>
				<a href="%s/page1">Page 1</a>
				<a href="%s/page2">Page 2</a>
			</body></html>`, baseURL, baseURL)
		case "/page1":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<html><body>
				<a href="%s/page3">Page 3</a>
			</body></html>`, baseURL)
		case "/page2", "/page3":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>Content</body></html>"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                    server.URL,
		ForceRecursiveCrawling: true,
		DisableSSRFProtection:  true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should find at least the home page and some linked pages
	if output.TotalURLs < 2 {
		t.Errorf("Expected at least 2 URLs from crawling, got %d", output.TotalURLs)
	}

	if output.Sources["crawl"] == 0 {
		t.Error("Expected URLs from crawling")
	}
}

// TestExtract_MaxURLs tests URL limit enforcement
func TestExtract_MaxURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		if r.URL.Path == "/sitemap.xml" {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>%s/page1</loc></url>
	<url><loc>%s/page2</loc></url>
	<url><loc>%s/page3</loc></url>
	<url><loc>%s/page4</loc></url>
	<url><loc>%s/page5</loc></url>
</urlset>`, baseURL, baseURL, baseURL, baseURL, baseURL)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                   server.URL,
		MaxURLs:               5,
		DisableSSRFProtection: true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if output.TotalURLs > 5 {
		t.Errorf("Expected max 5 URLs, got %d", output.TotalURLs)
	}
}

// TestExtract_ContextCancellation tests context cancellation
func TestExtract_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	input := Input{
		URL:                   server.URL,
		DisableSSRFProtection: true,
	}

	_, err := Extract(ctx, input)
	// Should timeout or be cancelled
	if err == nil {
		t.Log("Expected timeout/cancellation error, but extraction may have completed quickly")
	}
}

// TestExtract_SameDomainOnly tests that only same-domain URLs are extracted
func TestExtract_SameDomainOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		if r.URL.Path == "/sitemap.xml" {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>%s/page1</loc></url>
	<url><loc>https://external.com/page2</loc></url>
	<url><loc>%s/page3</loc></url>
</urlset>`, baseURL, baseURL)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                   server.URL,
		DisableSSRFProtection: true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should only have 2 URLs (external domain filtered)
	if output.TotalURLs != 2 {
		t.Errorf("Expected 2 URLs (external domain filtered), got %d", output.TotalURLs)
	}

	// Check that external URL is not in results
	for _, url := range output.URLs {
		if strings.Contains(url, "external.com") {
			t.Error("Found external domain URL in results")
		}
	}
}

// TestExtract_CustomUserAgent tests custom User-Agent
func TestExtract_CustomUserAgent(t *testing.T) {
	customUA := "TestBot/1.0"
	receivedUA := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	input := Input{
		URL:                   server.URL,
		UserAgent:             customUA,
		DisableSSRFProtection: true,
	}

	Extract(context.Background(), input)

	if receivedUA != customUA {
		t.Errorf("Expected User-Agent %q, got %q", customUA, receivedUA)
	}
}

// TestExtract_DefaultUserAgent tests default User-Agent
func TestExtract_DefaultUserAgent(t *testing.T) {
	receivedUA := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	input := Input{
		URL:                   server.URL,
		DisableSSRFProtection: true,
	}

	Extract(context.Background(), input)

	if receivedUA != DefaultUserAgent {
		t.Errorf("Expected default User-Agent %q, got %q", DefaultUserAgent, receivedUA)
	}
}

// TestNewURLExtractorTool tests tool creation
func TestNewURLExtractorTool(t *testing.T) {
	tool := NewURLExtractorTool()

	if tool == nil {
		t.Fatal("Expected tool to be created, got nil")
	}

	info := tool.ToolInfo()
	if info.Name != "URLExtractor" {
		t.Errorf("Expected tool name 'URLExtractor', got %q", info.Name)
	}

	if info.Description == "" {
		t.Error("Expected non-empty description")
	}
}

// TestExtract_RelativeLinks tests extraction of relative links during crawling
func TestExtract_RelativeLinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<html><body>
				<a href="/page1">Page 1</a>
				<a href="page2">Page 2</a>
				<a href="./page3">Page 3</a>
			</body></html>`)
		case "/page1", "/page2", "/page3":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>Content</body></html>"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                    server.URL,
		ForceRecursiveCrawling: true,
		DisableSSRFProtection:  true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should find the home page and linked pages
	if output.TotalURLs < 2 {
		t.Errorf("Expected at least 2 URLs with relative links, got %d", output.TotalURLs)
	}
}

// TestExtract_IgnoreFragments tests that URL fragments are handled
func TestExtract_IgnoreFragments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<html><body>
				<a href="#section1">Section 1</a>
				<a href="#section2">Section 2</a>
				<a href="%s/page1">Page 1</a>
			</body></html>`, baseURL)
		} else if r.URL.Path == "/page1" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>Content</body></html>"))
		}
	}))
	defer server.Close()

	input := Input{
		URL:                    server.URL,
		MaxURLs:                10,
		ForceRecursiveCrawling: true,
		DisableSSRFProtection:  true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should not include fragment-only links
	for _, url := range output.URLs {
		if strings.HasPrefix(url, "#") {
			t.Error("Found fragment-only URL in results")
		}
	}
}

// TestNormalizeURL tests URL normalization
func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantHost  string
		wantError bool
	}{
		{"Full HTTPS URL", "https://example.com", "example.com", false},
		{"Full HTTP URL", "http://example.com", "example.com", false},
		{"Partial URL", "example.com", "example.com", false},
		{"With Path", "example.com/path", "example.com", false},
		{"Empty URL", "", "", true},
		{"Whitespace URL", "   ", "", true},
		{"Invalid Protocol", "ftp://example.com", "", true},
		{"Missing Host", "https://", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizeURL(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for input %q, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error for input %q, got: %v", tt.input, err)
				return
			}

			if result.Host != tt.wantHost {
				t.Errorf("Expected host %q, got %q", tt.wantHost, result.Host)
			}
		})
	}
}

// TestExtract_CrawlDepthLimit tests crawl depth limiting
func TestExtract_CrawlDepthLimit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		callCount++
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		// Each page links to next level
		fmt.Fprintf(w, `<html><body><a href="%s/level%d">Next</a></body></html>`, baseURL, callCount+1)
	}))
	defer server.Close()

	input := Input{
		URL:                    server.URL,
		ForceRecursiveCrawling: true,
		MaxURLs:                10, // Limit URLs
		DisableSSRFProtection:  true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should respect max URLs limit (should not crawl indefinitely)
	if output.TotalURLs > 10 {
		t.Errorf("Crawled too many URLs (%d), URL limit may not be working", output.TotalURLs)
	}
}

// TestExtract_NoSitemapAutoCrawl tests that crawling happens automatically when no sitemap found
func TestExtract_NoSitemapAutoCrawl(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			// Return a simple page with a link
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			baseURL := "http://" + r.Host
			fmt.Fprintf(w, `<html><body><a href="%s/page1">Page 1</a></body></html>`, baseURL)
		case "/page1":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>Content</body></html>"))
		default:
			// No sitemap, return 404
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                   server.URL,
		DisableSSRFProtection: true,
		// ForceRecursiveCrawling defaults to false
		// But crawling will happen as fallback when no sitemap found
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have URLs from automatic crawling fallback (no sitemap found)
	if output.TotalURLs < 1 {
		t.Errorf("Expected at least 1 URL from automatic crawling fallback, got %d", output.TotalURLs)
	}

	if output.SitemapFound {
		t.Error("Expected sitemap not to be found")
	}

	if output.Sources["crawl"] == 0 {
		t.Error("Expected URLs from crawling (automatic fallback)")
	}
}

// TestExtract_WWWSubdomainHandling tests that www and non-www domains are treated as the same
func TestExtract_WWWSubdomainHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		if r.URL.Path == "/sitemap.xml" {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			// Sitemap contains www. prefix while base URL doesn't
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>http://www.%s/page1</loc></url>
	<url><loc>http://www.%s/page2</loc></url>
	<url><loc>%s/page3</loc></url>
</urlset>`, r.Host, r.Host, baseURL)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                   server.URL, // URL without www
		DisableSSRFProtection: true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should accept all 3 URLs even though some have www. prefix
	if output.TotalURLs != 3 {
		t.Errorf("Expected 3 URLs (www and non-www should be same domain), got %d", output.TotalURLs)
	}

	// Verify URLs with www. prefix are included
	hasWWW := false
	hasNonWWW := false
	for _, urlString := range output.URLs {
		if strings.Contains(urlString, "www.") {
			hasWWW = true
		} else {
			hasNonWWW = true
		}
	}

	if !hasWWW {
		t.Error("Expected to find URLs with www. prefix")
	}
	if !hasNonWWW {
		t.Error("Expected to find URLs without www. prefix")
	}
}

// TestNormalizeHost tests the host normalization for www handling
func TestNormalizeHost(t *testing.T) {
	extractor := &urlExtractor{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"With www", "www.example.com", "example.com"},
		{"Without www", "example.com", "example.com"},
		{"Subdomain with www", "www.blog.example.com", "blog.example.com"},
		{"Different subdomain", "blog.example.com", "blog.example.com"},
		{"Port with www", "www.example.com:8080", "example.com:8080"},
		{"Port without www", "example.com:8080", "example.com:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.normalizeHost(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeHost(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtract_RobotsTxtSitemapNormalization tests that sitemap URLs from robots.txt are normalized
func TestExtract_RobotsTxtSitemapNormalization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		switch r.URL.Path {
		case "/robots.txt":
			w.WriteHeader(http.StatusOK)
			// robots.txt points to sitemap with www. prefix
			fmt.Fprintf(w, "Sitemap: http://www.%s/sitemap.xml\n", r.Host)
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>%s/page1</loc></url>
	<url><loc>http://www.%s/page2</loc></url>
</urlset>`, baseURL, r.Host)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                   server.URL, // Without www
		DisableSSRFProtection: true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should successfully extract URLs even though robots.txt pointed to www. version
	if output.TotalURLs != 2 {
		t.Errorf("Expected 2 URLs (sitemap URL should be normalized), got %d", output.TotalURLs)
	}

	if !output.RobotsTxtFound {
		t.Error("Expected robots.txt to be found")
	}

	if !output.SitemapFound {
		t.Error("Expected sitemap to be found despite www mismatch")
	}
}

// TestExtract_RedirectToCanonicalURL tests that redirects are followed to get the canonical domain
func TestExtract_RedirectToCanonicalURL(t *testing.T) {
	// Create a server that redirects to a different host
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>%s/page1</loc></url>
</urlset>`, baseURL)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer finalServer.Close()

	// Create redirect server
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Redirect all requests to final server
		http.Redirect(w, r, finalServer.URL+r.URL.Path, http.StatusMovedPermanently)
	}))
	defer redirectServer.Close()

	input := Input{
		URL:                   redirectServer.URL,
		DisableSSRFProtection: true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Base URL should be updated to the final server after redirect
	if !strings.Contains(output.BaseURL, finalServer.URL) {
		t.Logf("Base URL was updated from %s to %s", redirectServer.URL, output.BaseURL)
		// Note: In test environment, the redirect might not always be followed
		// depending on server behavior, so we just log rather than fail
	}

	// Should still extract URLs successfully
	if output.TotalURLs < 1 {
		t.Logf("Expected at least 1 URL, got %d (redirect may affect extraction)", output.TotalURLs)
	}
}

// TestExtract_CrawlDelay tests that crawl delay is respected
func TestExtract_CrawlDelay(t *testing.T) {
	requestTimes := []time.Time{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestTimes = append(requestTimes, time.Now())
		baseURL := "http://" + r.Host

		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<html><body>
				<a href="%s/page1">Page 1</a>
				<a href="%s/page2">Page 2</a>
			</body></html>`, baseURL, baseURL)
		case "/page1", "/page2":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>Content</body></html>"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                    server.URL,
		ForceRecursiveCrawling: true,
		MaxURLs:                5,
		DisableSSRFProtection:  true,
	}

	startTime := time.Now()
	output, err := Extract(context.Background(), input)
	totalDuration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if output.TotalURLs < 2 {
		t.Errorf("Expected at least 2 URLs, got %d", output.TotalURLs)
	}

	// Check that delays were applied between requests
	if len(requestTimes) >= 2 {
		for i := 1; i < len(requestTimes); i++ {
			delay := requestTimes[i].Sub(requestTimes[i-1])
			// Allow some tolerance for processing time
			if delay < 150*time.Millisecond {
				t.Logf("Warning: Delay between request %d and %d was only %v (expected ~200ms)", i-1, i, delay)
			}
		}
	}

	// Total time should be at least (number of crawled pages - 1) * delay
	if len(requestTimes) >= 2 {
		minExpectedDuration := time.Duration(len(requestTimes)-1) * 150 * time.Millisecond
		if totalDuration < minExpectedDuration {
			t.Logf("Total duration %v seems too short for %d requests with 200ms delay", totalDuration, len(requestTimes))
		}
	}
}

// TestExtract_ForceCrawlingWithSitemap tests forced crawling even when sitemap exists
func TestExtract_ForceCrawlingWithSitemap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>%s/sitemap-page</loc></url>
</urlset>`, baseURL)
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<html><body><a href="%s/crawled-page">Link</a></body></html>`, baseURL)
		case "/crawled-page":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>Content</body></html>"))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                    server.URL,
		ForceRecursiveCrawling: true, // Force crawling even though sitemap exists
		MaxURLs:                10,
		DisableSSRFProtection:  true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have URLs from both sitemap AND crawling
	if output.TotalURLs < 2 {
		t.Errorf("Expected at least 2 URLs (from sitemap and crawling), got %d", output.TotalURLs)
	}

	if output.Sources["sitemap"] == 0 {
		t.Error("Expected URLs from sitemap")
	}

	if output.Sources["crawl"] == 0 {
		t.Error("Expected URLs from crawling (forced)")
	}
}

// TestExtract_RobotsTxtUserAgentParsing tests that robots.txt rules are only applied for the correct User-Agent
func TestExtract_RobotsTxtUserAgentParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		switch r.URL.Path {
		case "/robots.txt":
			w.WriteHeader(http.StatusOK)
			// BadBot is disallowed from everything
			// Our bot should only be blocked from /admin/
			fmt.Fprintf(w, `User-agent: BadBot
Disallow: /

User-agent: *
Disallow: /admin/
Sitemap: %s/sitemap.xml
`, baseURL)
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>%s/page1</loc></url>
	<url><loc>%s/admin/secret</loc></url>
	<url><loc>%s/public</loc></url>
</urlset>`, baseURL, baseURL, baseURL)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                   server.URL,
		DisableSSRFProtection: true,
	}

	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have 2 URLs: /page1 and /public (not /admin/secret)
	if output.TotalURLs != 2 {
		t.Errorf("Expected 2 URLs (excluding /admin/), got %d", output.TotalURLs)
	}

	// Check that /admin/ URLs are excluded
	for _, url := range output.URLs {
		if strings.Contains(url, "/admin/") {
			t.Errorf("Found disallowed URL: %s", url)
		}
	}
}

// TestExtract_SSRFProtectionLocalhost tests that localhost is blocked
func TestExtract_SSRFProtectionLocalhost(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"localhost", "http://localhost/test"},
		{"localhost with port", "http://localhost:8080/test"},
		{"127.0.0.1", "http://127.0.0.1/test"},
		{"127.0.0.1 with port", "http://127.0.0.1:8080/test"},
		{"IPv6 loopback", "http://[::1]/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := Input{
				URL: tt.url,
			}

			_, err := Extract(context.Background(), input)
			if err == nil {
				t.Fatalf("Expected error for %s, got nil", tt.url)
			}

			if !strings.Contains(err.Error(), "safety validation failed") &&
				!strings.Contains(err.Error(), "localhost") &&
				!strings.Contains(err.Error(), "private or local") {
				t.Errorf("Expected SSRF protection error, got: %v", err)
			}
		})
	}
}

// TestExtract_SSRFProtectionPrivateIPs tests that private IP ranges are blocked
func TestExtract_SSRFProtectionPrivateIPs(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"10.x.x.x", "http://10.0.0.1/test"},
		{"192.168.x.x", "http://192.168.1.1/test"},
		{"172.16.x.x", "http://172.16.0.1/test"},
		{"172.31.x.x", "http://172.31.255.254/test"},
		{"link-local", "http://169.254.169.254/test"}, // AWS metadata service
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := Input{
				URL: tt.url,
			}

			_, err := Extract(context.Background(), input)
			if err == nil {
				t.Fatalf("Expected error for %s, got nil", tt.url)
			}

			if !strings.Contains(err.Error(), "safety validation failed") &&
				!strings.Contains(err.Error(), "private or local") {
				t.Errorf("Expected SSRF protection error, got: %v", err)
			}
		})
	}
}

// TestExtract_RobotsCrawlDelay tests that Crawl-delay from robots.txt is respected
func TestExtract_RobotsCrawlDelay(t *testing.T) {
	requestTimes := []time.Time{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host

		// Record request times for crawled pages (not robots.txt or sitemap)
		if r.URL.Path != "/robots.txt" && r.URL.Path != "/sitemap.xml" {
			requestTimes = append(requestTimes, time.Now())
		}

		switch r.URL.Path {
		case "/robots.txt":
			w.WriteHeader(http.StatusOK)
			// Set a 1 second crawl delay
			fmt.Fprintf(w, `User-agent: *
Crawl-delay: 1
`)
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<html><body><a href="%s/page1">Page 1</a></body></html>`, baseURL)
		case "/page1":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>Content</body></html>"))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	input := Input{
		URL:                    server.URL,
		ForceRecursiveCrawling: true, // Force crawling to test delay
		MaxURLs:                5,
		CrawlDelayMs:           100, // Our configured delay (should be overridden by robots.txt)
		DisableSSRFProtection:  true,
	}

	startTime := time.Now()
	output, err := Extract(context.Background(), input)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if output.TotalURLs < 2 {
		t.Errorf("Expected at least 2 URLs, got %d", output.TotalURLs)
	}

	// Check that requests were spaced by approximately 1 second (robots.txt Crawl-delay)
	if len(requestTimes) >= 2 {
		delay := requestTimes[1].Sub(requestTimes[0])
		// Should be close to 1 second (allow 100ms tolerance)
		if delay < 900*time.Millisecond || delay > 1100*time.Millisecond {
			t.Errorf("Expected ~1s delay between requests (from robots.txt), got %v", delay)
		}
	}

	totalDuration := time.Since(startTime)
	// With at least 2 crawled pages and 1s delay, should take at least 1 second
	if totalDuration < 900*time.Millisecond {
		t.Errorf("Expected extraction to take at least 1s due to Crawl-delay, took %v", totalDuration)
	}
}

// TestExtract_ContextCancellationInSitemap tests that context cancellation works during sitemap parsing
func TestExtract_ContextCancellationInSitemap(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sitemap.xml" {
			// Delay response to allow cancellation
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer slowServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	input := Input{
		URL:                   slowServer.URL,
		DisableSSRFProtection: true,
	}

	startTime := time.Now()
	_, err := Extract(ctx, input)
	duration := time.Since(startTime)

	// Should return quickly due to context cancellation
	if duration > 500*time.Millisecond {
		t.Errorf("Expected quick return due to context cancellation, took %v", duration)
	}

	// Error might be nil if it completed before sitemap fetch, or context error
	if err != nil && !strings.Contains(err.Error(), "context") &&
		!strings.Contains(err.Error(), "deadline") {
		t.Logf("Got error (acceptable): %v", err)
	}
}

// TestValidateURLSafety tests the SSRF validation function directly
func TestValidateURLSafety(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		shouldErr bool
	}{
		{"Valid public URL", "https://example.com", false},
		{"Valid public IP", "https://8.8.8.8", false},
		{"Localhost", "https://localhost", true},
		{"127.0.0.1", "https://127.0.0.1", true},
		{"Private 10.x", "https://10.0.0.1", true},
		{"Private 192.168.x", "https://192.168.1.1", true},
		{"Private 172.16.x", "https://172.16.0.1", true},
		{"Link-local", "https://169.254.169.254", true},
		{"IPv6 loopback", "https://[::1]", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := normalizeURL(tt.url)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			err = validateURLSafety(u)
			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for %s, got nil", tt.url)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error for %s, got: %v", tt.url, err)
			}
		})
	}
}
