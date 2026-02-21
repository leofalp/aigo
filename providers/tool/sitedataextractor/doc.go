// Package sitedataextractor provides an AI tool that extracts structured
// company information from raw HTML pages using multiple deterministic strategies.
//
// Extraction relies on JSON-LD structured data, Schema.org markup, Open Graph
// meta tags, HTML link elements, and regex patterns applied to visible text.
// Each extracted value carries a confidence score and a source label so callers
// can evaluate reliability without re-parsing the HTML.
//
// The main entry point is [NewSiteDataExtractorTool], which returns a
// [tool.Tool] ready to be registered with an AIGO client. The underlying
// extraction logic is also available directly via [Extract] for use without
// the tool wrapper.
package sitedataextractor
