// Package urlextractor provides a tool for discovering and collecting all URLs
// from a target website. It works by inspecting robots.txt for sitemap references,
// parsing sitemap.xml files (including compressed and indexed sitemaps), and
// falling back to breadth-first recursive crawling when sitemaps are unavailable.
// Extracted URLs are deduplicated, filtered to the same domain, and automatically
// categorized into standard page types using multilingual pattern matching.
// The main entry point is [NewURLExtractorTool]; the underlying logic is exposed
// directly via [Extract] for programmatic use without the tool wrapper.
package urlextractor
