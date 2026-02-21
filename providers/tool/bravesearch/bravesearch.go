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

// NewBraveSearchTool returns a tool that searches the web via the Brave Search
// API and produces summarized results optimized for LLM consumption.
// Use [NewBraveSearchAdvancedTool] when the full structured response is needed.
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

// NewBraveSearchAdvancedTool returns a tool that searches the web via the Brave
// Search API and produces the complete structured response, including web, news,
// video, infobox, and location results without any summarisation.
// Use [NewBraveSearchTool] when a compact, LLM-friendly summary is sufficient.
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

// Input holds the query parameters forwarded to the Brave Search API.
// Query is the only required field; all other fields are optional filters
// that narrow results by count, region, language, safety level, or freshness.
type Input struct {
	Query      string `json:"query" jsonschema:"description=The search query string,required"`
	Count      int    `json:"count,omitempty" jsonschema:"description=Number of results to return (default: 10 max: 20)"`
	Country    string `json:"country,omitempty" jsonschema:"description=Country code for localized results (e.g. 'us' 'uk' 'de')"`
	SearchLang string `json:"search_lang,omitempty" jsonschema:"description=Search language code (e.g. 'en' 'es' 'fr')"`
	SafeSearch string `json:"safesearch,omitempty" jsonschema:"description=Safe search filter: 'off' 'moderate' or 'strict' (default: 'moderate')"`
	Freshness  string `json:"freshness,omitempty" jsonschema:"description=Time filter: 'pd' (past day) 'pw' (past week) 'pm' (past month) 'py' (past year)"`
}

// Output holds a summarized view of a Brave Search response, shaped for direct
// use by an LLM. It combines a human-readable Summary with the underlying
// Results slice so callers can inspect individual entries when needed.
type Output struct {
	Query   string         `json:"query" jsonschema:"description=The original search query"`
	Summary string         `json:"summary" jsonschema:"description=Formatted summary of search results"`
	Results []SearchResult `json:"results" jsonschema:"description=List of top search results"`
}

// SearchResult holds the title, URL, description snippet, and optional age of
// a single web result as returned by [Search].
type SearchResult struct {
	Title       string `json:"title" jsonschema:"description=Title of the result"`
	URL         string `json:"url" jsonschema:"description=URL of the result"`
	Description string `json:"description" jsonschema:"description=Description snippet of the result"`
	Age         string `json:"age,omitempty" jsonschema:"description=Age of the content (e.g. '2 hours ago')"`
}

// AdvancedOutput holds the full Brave Search API response mapped to typed Go
// structs. Each field corresponds to a distinct result category and is nil when
// the API returned no data for that category.
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

// WebResults holds the collection of organic web results returned by the API
// along with the result-set type identifier and family-friendliness flag.
type WebResults struct {
	Type           string      `json:"type"`
	Results        []WebResult `json:"results"`
	FamilyFriendly bool        `json:"family_friendly,omitempty"`
}

// WebResult holds the full metadata for a single organic web result, including
// title, URL, description, source locality flags, language, thumbnail, and
// optional deep-link buttons and schema.org markup.
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

// NewsResults holds the collection of news articles returned by the API
// along with the result-set type identifier.
type NewsResults struct {
	Type    string       `json:"type"`
	Results []NewsResult `json:"results"`
}

// NewsResult holds the metadata for a single news article, including title,
// URL, description, publication age, and an optional breaking-news flag.
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

// VideoResults holds the collection of video results returned by the API
// along with the result-set type identifier.
type VideoResults struct {
	Type    string        `json:"type"`
	Results []VideoResult `json:"results"`
}

// VideoResult holds the metadata for a single video result, including title,
// URL, description, age, and an optional [Video] struct with playback details.
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

// Video holds playback metadata for a video result such as duration, view
// count, creator, publisher, and whether a subscription is required to watch.
type Video struct {
	Duration             string  `json:"duration,omitempty"`
	Views                int64   `json:"views,omitempty"`
	Creator              string  `json:"creator,omitempty"`
	Publisher            string  `json:"publisher,omitempty"`
	RequiresSubscription bool    `json:"requires_subscription,omitempty"`
	Author               *Author `json:"author,omitempty"`
}

// Infobox holds structured knowledge-panel data about a named entity, including
// label, category, short and long descriptions, image, key-value attributes,
// social profiles, ratings, and data-source providers.
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

// LocationResults holds the collection of location results returned by the API
// along with the result-set type identifier.
type LocationResults struct {
	Type    string           `json:"type"`
	Results []LocationResult `json:"results"`
}

// LocationResult holds the metadata for a single location result, including
// title, URL, description, GPS coordinates, postal address, contact details,
// rating, distance, and zoom level for map rendering.
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

// MixedResultSection holds references to heterogeneous result types grouped by
// display position (main feed, top banner, or side panel). Each [MixedItem]
// carries a type discriminator and an index into the corresponding typed slice.
type MixedResultSection struct {
	Type string      `json:"type"`
	Main []MixedItem `json:"main,omitempty"`
	Top  []MixedItem `json:"top,omitempty"`
	Side []MixedItem `json:"side,omitempty"`
}

// MixedItem identifies a single entry within a [MixedResultSection] by its
// content type name and its position index in the corresponding result slice.
type MixedItem struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// Profile holds the display name, URL, long name, and avatar image of a source
// profile associated with a web result or infobox.
type Profile struct {
	Name     string `json:"name"`
	URL      string `json:"url,omitempty"`
	LongName string `json:"long_name,omitempty"`
	Img      string `json:"img,omitempty"`
}

// DeepLink holds the set of navigational deep-link buttons associated with a
// web result, allowing agents to jump directly to subsections of a page.
type DeepLink struct {
	Buttons []Button `json:"buttons,omitempty"`
}

// Button represents a single labeled navigational action within a [DeepLink],
// carrying a type discriminator, a human-readable title, and a target URL.
type Button struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// MetaURL holds the decomposed components of a result's canonical URL —
// scheme, network location, hostname, path, and favicon — as provided by the
// Brave API for display and routing purposes.
type MetaURL struct {
	Scheme   string `json:"scheme"`
	Netloc   string `json:"netloc"`
	Hostname string `json:"hostname"`
	Favicon  string `json:"favicon,omitempty"`
	Path     string `json:"path,omitempty"`
}

// Thumbnail holds the source URL and optional pixel dimensions of a preview
// image attached to a web, news, video, or location result.
type Thumbnail struct {
	Src    string `json:"src"`
	Height int    `json:"height,omitempty"`
	Width  int    `json:"width,omitempty"`
}

// Image holds the source URL and optional pixel dimensions of an image
// associated with an [Infobox] entity panel.
type Image struct {
	Src    string `json:"src"`
	Height int    `json:"height,omitempty"`
	Width  int    `json:"width,omitempty"`
}

// Author holds the name and optional profile URL of the creator of a video or
// other media result.
type Author struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Rating holds aggregate rating data for a result, including the numeric
// value, the maximum possible score, and the total review count. The optional
// Profile links the rating to a specific review platform.
type Rating struct {
	RatingValue float64  `json:"ratingValue,omitempty"`
	BestRating  float64  `json:"bestRating,omitempty"`
	ReviewCount int      `json:"reviewCount,omitempty"`
	Profile     *Profile `json:"profile,omitempty"`
	IsTricorder bool     `json:"is_tricorder,omitempty"`
}

// Postal holds the structured postal address of a location result, including
// street, city, region, postal code, and country fields.
type Postal struct {
	Type            string `json:"type,omitempty"`
	Country         string `json:"country,omitempty"`
	PostalCode      string `json:"postalCode,omitempty"`
	StreetAddress   string `json:"streetAddress,omitempty"`
	AddressRegion   string `json:"addressRegion,omitempty"`
	AddressLocality string `json:"addressLocality,omitempty"`
}

// Contact holds the email address and telephone number for a location result,
// both of which are optional and may be empty strings.
type Contact struct {
	Email     string `json:"email,omitempty"`
	Telephone string `json:"telephone,omitempty"`
}

// BraveAPIResponse is the top-level response envelope returned by the Brave
// Search REST API. It is mapped directly from JSON and then projected into
// either [Output] or [AdvancedOutput] depending on the calling tool.
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

// QueryInfo holds metadata about the submitted search query as reported by
// the Brave API, including the original text, whether spellcheck was bypassed,
// any automatically altered query, and whether a safe-search warning applies.
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

// Search performs a web search via the Brave Search API and returns an [Output]
// containing a plain-text summary and up to ten [SearchResult] entries. The
// summary also includes the infobox label and up to three news headlines when
// the API returns them. Returns an error if BRAVE_SEARCH_API_KEY is unset, the
// context is canceled, or the API responds with a non-200 status.
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

// SearchAdvanced performs a web search via the Brave Search API and returns the
// full [AdvancedOutput] without any summarisation. All result categories (web,
// news, video, infobox, locations, mixed) are included as typed structs with
// nil values for categories the API did not return. Returns an error if
// BRAVE_SEARCH_API_KEY is unset, the context is canceled, or the API responds
// with a non-200 status.
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
