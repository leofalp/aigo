package bravesearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/leofalp/aigo/core/cost"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/tool"
)

const (
	baseURL = "https://api.search.brave.com/res/v1"
)

// NewBraveSearchTool creates a new Brave Search tool for web search.
// Returns summarized results optimized for LLM consumption.
func NewBraveSearchTool() *tool.Tool[Input, Output] {
	return tool.NewTool[Input, Output](
		"BraveSearch",
		Search,
		tool.WithDescription("Search the web using Brave Search API. Provides high-quality, privacy-focused web search results. Works well for: current events, factual information, research queries, product information, and general web searches. Returns a summary of top results with titles, URLs, and descriptions. Requires BRAVE_SEARCH_API_KEY environment variable."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.005, // $5 per 1000 queries = $0.005 per query
			Currency:                "USD",
			CostDescription:         "per search query",
			Accuracy:                0.88, // Good accuracy - privacy-focused search results
			AverageDurationInMillis: 800,  // Average API response time (~800ms)
		}),
	)
}

// NewBraveSearchAdvancedTool creates a new Brave Search tool with detailed results.
// Returns complete structured data including all metadata.
func NewBraveSearchAdvancedTool() *tool.Tool[Input, AdvancedOutput] {
	return tool.NewTool[Input, AdvancedOutput](
		"BraveSearchAdvanced",
		SearchAdvanced,
		tool.WithDescription("Advanced web search using Brave Search API with complete structured results. Returns detailed information including web results, news, videos, FAQs, infoboxes, and more. Ideal when you need comprehensive search data with all metadata. Requires BRAVE_SEARCH_API_KEY environment variable."),
		tool.WithMetrics(cost.ToolMetrics{
			Amount:                  0.005, // $5 per 1000 queries = $0.005 per query
			Currency:                "USD",
			CostDescription:         "per search query",
			Accuracy:                0.90, // High accuracy with complete structured data
			AverageDurationInMillis: 900,  // Slightly slower due to more comprehensive data
		}),
	)
}

// Input represents the search query parameters
type Input struct {
	Query      string `json:"query" jsonschema:"description=The search query string,required"`
	Count      int    `json:"count,omitempty" jsonschema:"description=Number of results to return (default: 10 max: 20)"`
	Country    string `json:"country,omitempty" jsonschema:"description=Country code for localized results (e.g. 'us' 'uk' 'de')"`
	SearchLang string `json:"search_lang,omitempty" jsonschema:"description=Search language code (e.g. 'en' 'es' 'fr')"`
	SafeSearch string `json:"safesearch,omitempty" jsonschema:"description=Safe search filter: 'off' 'moderate' or 'strict' (default: 'moderate')"`
	Freshness  string `json:"freshness,omitempty" jsonschema:"description=Time filter: 'pd' (past day) 'pw' (past week) 'pm' (past month) 'py' (past year)"`
}

// Output represents a summarized search result optimized for LLM consumption
type Output struct {
	Query   string         `json:"query" jsonschema:"description=The original search query"`
	Summary string         `json:"summary" jsonschema:"description=Formatted summary of search results"`
	Results []SearchResult `json:"results" jsonschema:"description=List of top search results"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Title       string `json:"title" jsonschema:"description=Title of the result"`
	URL         string `json:"url" jsonschema:"description=URL of the result"`
	Description string `json:"description" jsonschema:"description=Description snippet of the result"`
	Age         string `json:"age,omitempty" jsonschema:"description=Age of the content (e.g. '2 hours ago')"`
}

// AdvancedOutput represents the complete Brave Search API response
type AdvancedOutput struct {
	Query        string              `json:"query" jsonschema:"description=The original search query"`
	Type         string              `json:"type" jsonschema:"description=Type of search response"`
	Web          *WebResults         `json:"web,omitempty" jsonschema:"description=Web search results"`
	News         *NewsResults        `json:"news,omitempty" jsonschema:"description=News results"`
	Videos       *VideoResults       `json:"videos,omitempty" jsonschema:"description=Video results"`
	Infobox      *Infobox            `json:"infobox,omitempty" jsonschema:"description=Infobox with structured information"`
	Locations    *LocationResults    `json:"locations,omitempty" jsonschema:"description=Location/map results"`
	MixedResults *MixedResultSection `json:"mixed,omitempty" jsonschema:"description=Mixed content results"`
}

// WebResults contains web search results
type WebResults struct {
	Type           string      `json:"type"`
	Results        []WebResult `json:"results"`
	FamilyFriendly bool        `json:"family_friendly,omitempty"`
}

// WebResult represents a web search result
type WebResult struct {
	Title          string     `json:"title"`
	URL            string     `json:"url"`
	IsSourceLocal  bool       `json:"is_source_local"`
	IsSourceBoth   bool       `json:"is_source_both"`
	Description    string     `json:"description"`
	PageAge        string     `json:"page_age,omitempty"`
	PageFetched    string     `json:"page_fetched,omitempty"`
	Profile        *Profile   `json:"profile,omitempty"`
	Language       string     `json:"language,omitempty"`
	FamilyFriendly bool       `json:"family_friendly,omitempty"`
	ExtraSnippets  []string   `json:"extra_snippets,omitempty"`
	DeepResults    *DeepLink  `json:"deep_results,omitempty"`
	SchemaMarkup   []byte     `json:"schema_org,omitempty"`
	Thumbnail      *Thumbnail `json:"thumbnail,omitempty"`
	Age            string     `json:"age,omitempty"`
	MetaURL        *MetaURL   `json:"meta_url,omitempty"`
}

// NewsResults contains news search results
type NewsResults struct {
	Type    string       `json:"type"`
	Results []NewsResult `json:"results"`
}

// NewsResult represents a news article result
type NewsResult struct {
	Title        string     `json:"title"`
	URL          string     `json:"url"`
	Description  string     `json:"description,omitempty"`
	Age          string     `json:"age"`
	PageAge      string     `json:"page_age"`
	BreakingNews bool       `json:"breaking,omitempty"`
	MetaURL      *MetaURL   `json:"meta_url,omitempty"`
	Thumbnail    *Thumbnail `json:"thumbnail,omitempty"`
}

// VideoResults contains video search results
type VideoResults struct {
	Type    string        `json:"type"`
	Results []VideoResult `json:"results"`
}

// VideoResult represents a video search result
type VideoResult struct {
	Type        string     `json:"type"`
	URL         string     `json:"url"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Age         string     `json:"age,omitempty"`
	PageAge     string     `json:"page_age,omitempty"`
	Video       *Video     `json:"video,omitempty"`
	MetaURL     *MetaURL   `json:"meta_url,omitempty"`
	Thumbnail   *Thumbnail `json:"thumbnail,omitempty"`
}

// Video contains video metadata
type Video struct {
	Duration             string  `json:"duration,omitempty"`
	Views                int64   `json:"views,omitempty"`
	Creator              string  `json:"creator,omitempty"`
	Publisher            string  `json:"publisher,omitempty"`
	RequiresSubscription bool    `json:"requires_subscription,omitempty"`
	Author               *Author `json:"author,omitempty"`
}

// Infobox contains structured information about an entity
type Infobox struct {
	Type       string            `json:"type"`
	Position   int               `json:"position"`
	Label      string            `json:"label,omitempty"`
	Category   string            `json:"category,omitempty"`
	LongDesc   string            `json:"long_desc,omitempty"`
	ShortDesc  string            `json:"short_desc,omitempty"`
	Image      *Image            `json:"image,omitempty"`
	Attributes [][]string        `json:"attributes,omitempty"`
	Profiles   []Profile         `json:"profiles,omitempty"`
	Website    string            `json:"website,omitempty"`
	Ratings    []Rating          `json:"ratings,omitempty"`
	Providers  map[string]string `json:"providers,omitempty"`
}

// LocationResults contains location/map results
type LocationResults struct {
	Type    string           `json:"type"`
	Results []LocationResult `json:"results"`
}

// LocationResult represents a location result
type LocationResult struct {
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	URL         string     `json:"url,omitempty"`
	Description string     `json:"description,omitempty"`
	Coordinates []float64  `json:"coordinates,omitempty"`
	Postal      *Postal    `json:"postal_address,omitempty"`
	Contact     *Contact   `json:"contact,omitempty"`
	Thumbnail   *Thumbnail `json:"thumbnail,omitempty"`
	Rating      *Rating    `json:"rating,omitempty"`
	Distance    string     `json:"distance,omitempty"`
	Zoom        string     `json:"zoom_level,omitempty"`
}

// MixedResultSection contains mixed content types
type MixedResultSection struct {
	Type string      `json:"type"`
	Main []MixedItem `json:"main,omitempty"`
	Top  []MixedItem `json:"top,omitempty"`
	Side []MixedItem `json:"side,omitempty"`
}

// MixedItem represents a mixed content item
type MixedItem struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// Supporting types
type Profile struct {
	Name     string `json:"name"`
	URL      string `json:"url,omitempty"`
	LongName string `json:"long_name,omitempty"`
	Img      string `json:"img,omitempty"`
}

type DeepLink struct {
	Buttons []Button `json:"buttons,omitempty"`
}

type Button struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type MetaURL struct {
	Scheme   string `json:"scheme"`
	Netloc   string `json:"netloc"`
	Hostname string `json:"hostname"`
	Favicon  string `json:"favicon,omitempty"`
	Path     string `json:"path,omitempty"`
}

type Thumbnail struct {
	Src    string `json:"src"`
	Height int    `json:"height,omitempty"`
	Width  int    `json:"width,omitempty"`
}

type Image struct {
	Src    string `json:"src"`
	Height int    `json:"height,omitempty"`
	Width  int    `json:"width,omitempty"`
}

type Author struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type Rating struct {
	RatingValue float64  `json:"ratingValue,omitempty"`
	BestRating  float64  `json:"bestRating,omitempty"`
	ReviewCount int      `json:"reviewCount,omitempty"`
	Profile     *Profile `json:"profile,omitempty"`
	IsTricorder bool     `json:"is_tricorder,omitempty"`
}

type Postal struct {
	Type            string `json:"type,omitempty"`
	Country         string `json:"country,omitempty"`
	PostalCode      string `json:"postalCode,omitempty"`
	StreetAddress   string `json:"streetAddress,omitempty"`
	AddressRegion   string `json:"addressRegion,omitempty"`
	AddressLocality string `json:"addressLocality,omitempty"`
}

type Contact struct {
	Email     string `json:"email,omitempty"`
	Telephone string `json:"telephone,omitempty"`
}

// BraveAPIResponse is the internal response structure from Brave API
type BraveAPIResponse struct {
	Type      string              `json:"type"`
	Query     *QueryInfo          `json:"query,omitempty"`
	Web       *WebResults         `json:"web,omitempty"`
	News      *NewsResults        `json:"news,omitempty"`
	Videos    *VideoResults       `json:"videos,omitempty"`
	Infobox   *Infobox            `json:"infobox,omitempty"`
	Locations *LocationResults    `json:"locations,omitempty"`
	Mixed     *MixedResultSection `json:"mixed,omitempty"`
}

type QueryInfo struct {
	Original          string `json:"original"`
	SpellcheckOff     bool   `json:"spellcheck_off,omitempty"`
	ShowStrictWarning bool   `json:"show_strict_warning,omitempty"`
	AlteredQuery      string `json:"altered,omitempty"`
}

// fetchBraveSearchResults performs the API call to Brave Search
func fetchBraveSearchResults(ctx context.Context, input Input) (*BraveAPIResponse, error) {
	apiKey := os.Getenv("BRAVE_SEARCH_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("BRAVE_SEARCH_API_KEY environment variable is not set")
	}

	// Build query parameters
	params := url.Values{}
	params.Add("q", input.Query)

	// Set defaults and optional parameters
	count := input.Count
	if count <= 0 {
		count = 10
	}
	if count > 20 {
		count = 20
	}
	params.Add("count", fmt.Sprintf("%d", count))

	if input.Country != "" {
		params.Add("country", input.Country)
	}
	if input.SearchLang != "" {
		params.Add("search_lang", input.SearchLang)
	}
	if input.SafeSearch != "" {
		params.Add("safesearch", input.SafeSearch)
	}
	if input.Freshness != "" {
		params.Add("freshness", input.Freshness)
	}

	// Add result filters to get comprehensive results
	params.Add("result_filter", "web,news,videos,infobox")

	fullURL := fmt.Sprintf("%s/web/search?%s", baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer utils.CloseWithLog(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unexpected status code %d (failed to read error body: %w)", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	// Go's http.Client automatically handles gzip decompression when
	// Accept-Encoding is not manually set
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var apiResponse BraveAPIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return &apiResponse, nil
}

// Search performs a web search and returns a summarized result optimized for LLMs
func Search(ctx context.Context, input Input) (Output, error) {
	apiResponse, err := fetchBraveSearchResults(ctx, input)
	if err != nil {
		return Output{}, err
	}

	// Extract results
	var results []SearchResult
	var summaryParts []string

	// Add web results
	if apiResponse.Web != nil && len(apiResponse.Web.Results) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("Found %d web results:", len(apiResponse.Web.Results)))
		for i, webResult := range apiResponse.Web.Results {
			if i >= 10 { // Limit to top 10 for summary
				break
			}

			// Clean description (remove HTML tags)
			desc := cleanHTML(webResult.Description)

			result := SearchResult{
				Title:       webResult.Title,
				URL:         webResult.URL,
				Description: desc,
				Age:         webResult.Age,
			}
			results = append(results, result)

			summaryParts = append(summaryParts, fmt.Sprintf("\n%d. %s\n   URL: %s\n   %s",
				i+1, result.Title, result.URL, truncate(desc, 200)))
		}
	}

	// Add infobox if available
	if apiResponse.Infobox != nil {
		summaryParts = append(summaryParts, fmt.Sprintf("\n\nInfobox: %s", apiResponse.Infobox.Label))
		if apiResponse.Infobox.ShortDesc != "" {
			summaryParts = append(summaryParts, fmt.Sprintf("Description: %s", apiResponse.Infobox.ShortDesc))
		}
	}

	// Add news if available
	if apiResponse.News != nil && len(apiResponse.News.Results) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("\n\nRecent news (%d articles):", len(apiResponse.News.Results)))
		for i, news := range apiResponse.News.Results {
			if i >= 3 { // Limit to top 3 news
				break
			}
			summaryParts = append(summaryParts, fmt.Sprintf("- %s (%s)", news.Title, news.Age))
		}
	}

	summary := strings.Join(summaryParts, "\n")
	if summary == "" {
		summary = fmt.Sprintf("No results found for '%s'. Try a different query or check your spelling.", input.Query)
	}

	return Output{
		Query:   input.Query,
		Summary: summary,
		Results: results,
	}, nil
}

// SearchAdvanced performs a web search and returns complete structured results
func SearchAdvanced(ctx context.Context, input Input) (AdvancedOutput, error) {
	apiResponse, err := fetchBraveSearchResults(ctx, input)
	if err != nil {
		return AdvancedOutput{}, err
	}

	return AdvancedOutput{
		Query:        input.Query,
		Type:         apiResponse.Type,
		Web:          apiResponse.Web,
		News:         apiResponse.News,
		Videos:       apiResponse.Videos,
		Infobox:      apiResponse.Infobox,
		Locations:    apiResponse.Locations,
		MixedResults: apiResponse.Mixed,
	}, nil
}

// cleanHTML removes HTML tags from a string
func cleanHTML(s string) string {
	// Simple HTML tag removal (for production, consider using a proper HTML parser)
	s = strings.ReplaceAll(s, "<strong>", "")
	s = strings.ReplaceAll(s, "</strong>", "")
	s = strings.ReplaceAll(s, "<em>", "")
	s = strings.ReplaceAll(s, "</em>", "")
	s = strings.ReplaceAll(s, "<b>", "")
	s = strings.ReplaceAll(s, "</b>", "")
	return s
}

// truncate truncates a string to a maximum length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
