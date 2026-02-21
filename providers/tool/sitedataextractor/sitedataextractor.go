package sitedataextractor

import (
	"context"
	"net/url"
	"strings"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/providers/tool"
	"github.com/leofalp/aigo/providers/tool/urlextractor"
)

// NewSiteDataExtractorTool creates a new site data extraction tool that extracts
// structured company information from HTML pages.
//
// This tool performs deterministic extraction of business data including:
//   - Company identity (name, VAT, tax code)
//   - Contact information (email, phone, fax, PEC)
//   - Location data (address, city, province, country)
//   - Online presence (website, logo, social media links)
//   - Business metrics (employees, turnover, ATECO code)
//
// The extraction uses multiple methods (JSON-LD, Schema.org, meta tags, regex patterns)
// and provides confidence scores for each extracted field.
//
// Example:
//
//	tool := sitedataextractor.NewSiteDataExtractorTool()
//	client, _ := client.New(provider, client.WithTools(tool))
func NewSiteDataExtractorTool() *tool.Tool[Input, Output] {
	return tool.NewTool[Input, Output](
		"SiteDataExtractor",
		Extract,
		tool.WithDescription("Extracts structured company data from HTML pages including identity (name, VAT, tax code), contact information (email, phone, PEC), location data (address, city, country), online presence (logo, social links), and business metrics (employees, turnover). Uses JSON-LD, Schema.org, meta tags, and regex patterns for extraction. Returns confidence scores for each field."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.0, // Free - local HTML parsing
			Currency:                "USD",
			CostDescription:         "local HTML parsing",
			Accuracy:                0.80, // Depends on page structure
			AverageDurationInMillis: 50,   // Fast local processing
		}),
	)
}

// Input represents the input parameters for the site data extractor tool.
type Input struct {
	// SiteStructure contains the output from urlextractor including base URL and standard pages.
	SiteStructure urlextractor.Output `json:"site_structure" jsonschema:"description=Output from URLExtractor tool containing base URL and site structure,required"`

	// Pages contains the HTML content of fetched pages, keyed by page category.
	// Example: {"home": "<html>...", "contact": "<html>...", "about": "<html>..."}
	Pages map[string]string `json:"pages" jsonschema:"description=HTML content of fetched pages keyed by category (home contact about privacy etc.),required"`
}

// Output represents the extracted site data with confidence scores.
type Output struct {
	// === IDENTITY ===
	CompanyName ExtractedField `json:"company_name" jsonschema:"description=Company or organization name"`
	VAT         ExtractedField `json:"vat" jsonschema:"description=VAT number (numeric part only)"`
	TaxCode     ExtractedField `json:"tax_code" jsonschema:"description=Tax/Fiscal code (e.g. Italian Codice Fiscale - 16 chars)"`

	// === LOCATION ===
	Address     ExtractedField `json:"address" jsonschema:"description=Street address with number"`
	ZipCode     ExtractedField `json:"zip_code" jsonschema:"description=Postal/ZIP code"`
	City        ExtractedField `json:"city" jsonschema:"description=City name"`
	Region      ExtractedField `json:"region" jsonschema:"description=Full region name"`
	CountryCode ExtractedField `json:"country_code" jsonschema:"description=ISO country code (e.g. IT US DE)"`
	State       ExtractedField `json:"state" jsonschema:"description=Full country name (e.g. Italy United States)"`

	// === CONTACT ===
	Phone ExtractedField `json:"phone" jsonschema:"description=Phone number"`
	Fax   ExtractedField `json:"fax" jsonschema:"description=Fax number"`
	Email ExtractedField `json:"email" jsonschema:"description=General contact email"`
	PEC   ExtractedField `json:"pec" jsonschema:"description=Certified email (PEC - Italy specific)"`

	// === ONLINE PRESENCE ===
	Website    ExtractedField `json:"website" jsonschema:"description=Main website URL"`
	LogoURL    ExtractedField `json:"logo_url" jsonschema:"description=Direct URL to company logo image"`
	Facebook   ExtractedField `json:"facebook" jsonschema:"description=Facebook page URL"`
	LinkedIn   ExtractedField `json:"linkedin" jsonschema:"description=LinkedIn company page URL"`
	LinkedInID ExtractedField `json:"linkedin_id" jsonschema:"description=LinkedIn numeric company ID (if extractable from URL)"`
	Twitter    ExtractedField `json:"twitter" jsonschema:"description=Twitter/X profile URL"`
	Instagram  ExtractedField `json:"instagram" jsonschema:"description=Instagram profile URL"`
	YouTube    ExtractedField `json:"youtube" jsonschema:"description=YouTube channel URL"`

	// === BUSINESS DATA ===
	AtecoCode          ExtractedField `json:"ateco_code" jsonschema:"description=ATECO/NACE activity code (e.g. 29.10.00)"`
	Employees          ExtractedField `json:"employees" jsonschema:"description=Number of employees"`
	YearlyTurnover     ExtractedField `json:"yearly_turnover" jsonschema:"description=Annual turnover/revenue"`
	DateOfBalance      ExtractedField `json:"date_of_balance" jsonschema:"description=Date of last financial statement (YYYY-MM-DD)"`
	Industry           ExtractedField `json:"industry" jsonschema:"description=Industry/sector"`
	Description        ExtractedField `json:"description" jsonschema:"description=Company description"`
	ProductDescription ExtractedField `json:"product_description" jsonschema:"description=Description of products or services offered"`

	// === METADATA ===
	OverallConfidence float64  `json:"overall_confidence" jsonschema:"description=Weighted average confidence score (0-1)"`
	FieldsExtracted   int      `json:"fields_extracted" jsonschema:"description=Number of fields with non-empty values"`
	FieldsTotal       int      `json:"fields_total" jsonschema:"description=Total number of extractable fields"`
	SourcesUsed       []string `json:"sources_used" jsonschema:"description=List of extraction methods used (json-ld meta-tags regex etc.)"`
}

// ExtractedField represents a single extracted value with metadata.
type ExtractedField struct {
	// Value is the extracted value (empty string if not found).
	Value string `json:"value" jsonschema:"description=Extracted value (empty if not found)"`

	// Confidence is the confidence score from 0 to 1.
	Confidence float64 `json:"confidence" jsonschema:"description=Confidence score between 0 and 1"`

	// Source indicates the extraction method used.
	// Possible values: "json-ld", "schema-org", "og:tag", "meta-tag", "link-href", "regex", "dom"
	Source string `json:"source" jsonschema:"description=Extraction method used (json-ld schema-org og:tag meta-tag link-href regex dom)"`

	// PageSource indicates which page the value was extracted from.
	// Possible values: "home", "contact", "about", "privacy", etc.
	PageSource string `json:"page_source" jsonschema:"description=Page category where value was found (home contact about privacy etc.)"`

	// Candidates contains alternative values found (if multiple matches).
	Candidates []string `json:"candidates,omitempty" jsonschema:"description=Alternative values found when multiple matches exist"`
}

// Extract performs deterministic, multi-pass extraction of structured company
// data from the HTML pages supplied in input.Pages.
//
// Each page is parsed once and then passed through a pipeline of specialised
// extractors (logo, contacts, social links, address, company identity, business
// metrics, and product description). Results from higher-confidence sources
// (e.g. JSON-LD) take precedence over lower-confidence ones (e.g. regex).
// Alternative values found during extraction are preserved in each field's
// Candidates list.
//
// The website field is always populated from input.SiteStructure.BaseURL when
// no website is found in the page content. Overall confidence and extracted
// field counts are calculated and stored in the returned Output.
//
// Extract never returns a non-nil error under normal operation; the error
// return exists to satisfy the tool function signature.
func Extract(ctx context.Context, input Input) (Output, error) {
	output := Output{
		FieldsTotal: 24, // Total extractable fields (removed ProvinceCode)
		SourcesUsed: make([]string, 0),
	}

	// Parse base URL for domain validation
	baseURL, err := url.Parse(input.SiteStructure.BaseURL)
	if err != nil {
		// Use empty URL if parsing fails
		baseURL = &url.URL{}
	}

	// Track which extraction sources were used
	sourcesUsed := make(map[string]bool)

	// Create extractor context with shared data
	extractCtx := &extractionContext{
		baseURL:     baseURL,
		pages:       input.Pages,
		sourcesUsed: sourcesUsed,
	}

	// Extract all data from each page
	for pageCategory, htmlContent := range input.Pages {
		if htmlContent == "" {
			continue
		}

		// Parse the HTML once for this page
		pageData := parseHTMLPage(htmlContent, baseURL)
		pageData.category = pageCategory

		// Run all extractors on this page
		extractLogo(extractCtx, pageData, &output)
		extractContacts(extractCtx, pageData, &output)
		extractSocialLinks(extractCtx, pageData, &output)
		extractAddress(extractCtx, pageData, &output)
		extractCompanyData(extractCtx, pageData, &output)
		extractBusinessMetrics(extractCtx, pageData, &output)
		extractProductDescription(extractCtx, pageData, &output)
	}

	// Set website from base URL
	if output.Website.Value == "" && input.SiteStructure.BaseURL != "" {
		output.Website = ExtractedField{
			Value:      input.SiteStructure.BaseURL,
			Confidence: 1.0,
			Source:     "urlextractor",
			PageSource: "input",
		}
		sourcesUsed["urlextractor"] = true
	}

	// Collect sources used
	for source := range sourcesUsed {
		output.SourcesUsed = append(output.SourcesUsed, source)
	}

	// Count extracted fields and calculate overall confidence
	output.FieldsExtracted, output.OverallConfidence = calculateMetrics(&output)

	return output, nil
}

// extractionContext holds shared data for extraction operations.
type extractionContext struct {
	baseURL     *url.URL
	pages       map[string]string
	sourcesUsed map[string]bool
}

// pageData holds parsed data from a single HTML page.
type pageData struct {
	category string
	html     string
	jsonLD   []map[string]interface{}
	metaTags map[string]string // og:*, meta name/property
	links    []linkTag
	scripts  []string
	text     string // visible text content
}

// linkTag represents a parsed <link> tag.
type linkTag struct {
	rel   string
	href  string
	typ   string
	sizes string
}

// updateFieldIfBetter updates a field only if the new value has higher confidence
// or if the current value is empty.
func updateFieldIfBetter(current *ExtractedField, new ExtractedField) {
	if new.Value == "" {
		return
	}

	// Add to candidates if different from current value and not already a candidate
	if current.Value != "" && current.Value != new.Value {
		if current.Candidates == nil {
			current.Candidates = []string{current.Value}
		}
		// Only add if not already in candidates (deduplication)
		if !containsCandidate(current.Candidates, new.Value) {
			current.Candidates = append(current.Candidates, new.Value)
		}
	}

	// Update if current is empty or new has higher confidence
	if current.Value == "" || new.Confidence > current.Confidence {
		new.Candidates = current.Candidates // Preserve candidates
		*current = new
	}
}

// containsCandidate checks if a value already exists in the candidates slice.
func containsCandidate(candidates []string, value string) bool {
	for _, c := range candidates {
		if c == value {
			return true
		}
	}
	return false
}

// calculateMetrics counts extracted fields and calculates weighted overall confidence.
func calculateMetrics(output *Output) (int, float64) {
	// Field weights for confidence calculation
	weights := map[string]float64{
		"company_name": 2.0,
		"email":        1.5,
		"phone":        1.5,
		"website":      1.0,
		"logo_url":     1.0,
		"address":      1.2,
		"vat":          1.3,
		"city":         1.0,
		"country_code": 0.8,
	}

	fields := []struct {
		name  string
		field ExtractedField
	}{
		{"company_name", output.CompanyName},
		{"vat", output.VAT},
		{"tax_code", output.TaxCode},
		{"address", output.Address},
		{"zip_code", output.ZipCode},
		{"city", output.City},
		{"region", output.Region},
		{"country_code", output.CountryCode},
		{"state", output.State},
		{"phone", output.Phone},
		{"fax", output.Fax},
		{"email", output.Email},
		{"pec", output.PEC},
		{"website", output.Website},
		{"logo_url", output.LogoURL},
		{"facebook", output.Facebook},
		{"linkedin", output.LinkedIn},
		{"linkedin_id", output.LinkedInID},
		{"twitter", output.Twitter},
		{"instagram", output.Instagram},
		{"youtube", output.YouTube},
		{"ateco_code", output.AtecoCode},
		{"employees", output.Employees},
		{"yearly_turnover", output.YearlyTurnover},
	}

	count := 0
	totalWeight := 0.0
	weightedSum := 0.0

	for _, f := range fields {
		if f.field.Value != "" {
			count++
			w := weights[f.name]
			if w == 0 {
				w = 1.0 // Default weight
			}
			weightedSum += f.field.Confidence * w
			totalWeight += w
		}
	}

	overallConfidence := 0.0
	if totalWeight > 0 {
		overallConfidence = weightedSum / totalWeight
	}

	return count, overallConfidence
}

// isSameDomain checks if a URL belongs to the same domain as the base URL.
func isSameDomain(urlStr string, baseURL *url.URL) bool {
	if baseURL == nil || baseURL.Host == "" {
		return true // Accept if no base URL to compare
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Normalize hosts by removing www prefix
	baseHost := strings.TrimPrefix(baseURL.Host, "www.")
	urlHost := strings.TrimPrefix(parsed.Host, "www.")

	return baseHost == urlHost
}
