package exa

// === SEARCH INPUT/OUTPUT ===

// SearchInput holds all parameters accepted by the Exa semantic search
// endpoints. Query is the only required field; all other fields are optional
// filters that narrow or enrich the results returned by the API.
type SearchInput struct {
	Query              string   `json:"query" jsonschema:"description=The search query to perform,required"`
	Type               string   `json:"type,omitempty" jsonschema:"description=Search type: neural (embedding-based) keyword (keyword-based) or auto (default - automatically selects best type),enum=neural,enum=keyword,enum=auto"`
	NumResults         int      `json:"num_results,omitempty" jsonschema:"description=Number of results to return (default: 10 max: 100),minimum=1,maximum=100"`
	IncludeDomains     []string `json:"include_domains,omitempty" jsonschema:"description=List of domains to include in search results"`
	ExcludeDomains     []string `json:"exclude_domains,omitempty" jsonschema:"description=List of domains to exclude from search results"`
	StartPublishedDate string   `json:"start_published_date,omitempty" jsonschema:"description=Filter results published after this date (ISO 8601 format YYYY-MM-DD)"`
	EndPublishedDate   string   `json:"end_published_date,omitempty" jsonschema:"description=Filter results published before this date (ISO 8601 format YYYY-MM-DD)"`
	StartCrawlDate     string   `json:"start_crawl_date,omitempty" jsonschema:"description=Filter results crawled after this date (ISO 8601 format)"`
	EndCrawlDate       string   `json:"end_crawl_date,omitempty" jsonschema:"description=Filter results crawled before this date (ISO 8601 format)"`
	Category           string   `json:"category,omitempty" jsonschema:"description=Category filter for focused results,enum=company,enum=research paper,enum=news,enum=pdf,enum=github,enum=tweet,enum=personal site,enum=financial report,enum=people"`
	IncludeText        bool     `json:"include_text,omitempty" jsonschema:"description=Include full page text content in results"`
	IncludeHighlights  bool     `json:"include_highlights,omitempty" jsonschema:"description=Include key sentence highlights in results"`
}

// SearchOutput contains the summarized result of a [Search] call, formatted for
// efficient LLM consumption. Results contains at most maxSummaryResults entries
// even when the API returns more; the full count is visible in Summary.
type SearchOutput struct {
	Query   string         `json:"query" jsonschema:"description=The original search query"`
	Summary string         `json:"summary" jsonschema:"description=Formatted summary of search results"`
	Results []SearchResult `json:"results" jsonschema:"description=List of search results"`
}

// SearchResult represents a single item returned by a [Search] or
// [FindSimilar] call. Text and Highlights are only populated when the
// corresponding IncludeText or IncludeHighlights flags are set on the input.
type SearchResult struct {
	Title         string   `json:"title" jsonschema:"description=Title of the result"`
	URL           string   `json:"url" jsonschema:"description=URL of the result"`
	PublishedDate string   `json:"published_date,omitempty" jsonschema:"description=Publication date of the content"`
	Author        string   `json:"author,omitempty" jsonschema:"description=Author of the content"`
	Text          string   `json:"text,omitempty" jsonschema:"description=Full text content if requested"`
	Highlights    []string `json:"highlights,omitempty" jsonschema:"description=Key sentence highlights if requested"`
}

// SearchAdvancedOutput contains the full structured response from a
// [SearchAdvanced] call, including all metadata fields and the resolved
// search type chosen by the Exa API.
type SearchAdvancedOutput struct {
	Query              string                 `json:"query" jsonschema:"description=The original search query"`
	Results            []SearchResultAdvanced `json:"results" jsonschema:"description=List of detailed search results"`
	ResolvedSearchType string                 `json:"resolved_search_type,omitempty" jsonschema:"description=The actual search type used"`
	RequestID          string                 `json:"request_id,omitempty" jsonschema:"description=Unique request identifier"`
}

// SearchResultAdvanced represents a single item from a [SearchAdvanced] call
// with all available metadata, relevance scores, and optional content fields.
type SearchResultAdvanced struct {
	ID              string    `json:"id" jsonschema:"description=Temporary ID for the document"`
	Title           string    `json:"title" jsonschema:"description=Title of the result"`
	URL             string    `json:"url" jsonschema:"description=URL of the result"`
	Score           float64   `json:"score,omitempty" jsonschema:"description=Relevance score of the result"`
	PublishedDate   string    `json:"published_date,omitempty" jsonschema:"description=Publication date of the content"`
	Author          string    `json:"author,omitempty" jsonschema:"description=Author of the content"`
	Text            string    `json:"text,omitempty" jsonschema:"description=Full text content if requested"`
	Highlights      []string  `json:"highlights,omitempty" jsonschema:"description=Key sentence highlights if requested"`
	HighlightScores []float64 `json:"highlight_scores,omitempty" jsonschema:"description=Scores for each highlight"`
	Summary         string    `json:"summary,omitempty" jsonschema:"description=AI-generated summary if requested"`
}

// === FIND SIMILAR INPUT/OUTPUT ===

// SimilarInput holds the parameters for a [FindSimilar] call. Either URL or
// Text must be non-empty; supplying both sends both fields to the API and the
// API determines how to weight them for similarity matching.
type SimilarInput struct {
	URL               string   `json:"url,omitempty" jsonschema:"description=URL to find similar content for (provide either url or text)"`
	Text              string   `json:"text,omitempty" jsonschema:"description=Text to find similar content for (provide either url or text)"`
	NumResults        int      `json:"num_results,omitempty" jsonschema:"description=Number of results to return (default: 10 max: 100),minimum=1,maximum=100"`
	IncludeDomains    []string `json:"include_domains,omitempty" jsonschema:"description=List of domains to include in results"`
	ExcludeDomains    []string `json:"exclude_domains,omitempty" jsonschema:"description=List of domains to exclude from results"`
	IncludeText       bool     `json:"include_text,omitempty" jsonschema:"description=Include full page text content in results"`
	IncludeHighlights bool     `json:"include_highlights,omitempty" jsonschema:"description=Include key sentence highlights in results"`
}

// SimilarOutput contains the result of a [FindSimilar] call, including the
// source used for similarity matching and a formatted summary of similar pages.
type SimilarOutput struct {
	Source  string         `json:"source" jsonschema:"description=The source used for similarity search (URL or text-based input indicator)"`
	Summary string         `json:"summary" jsonschema:"description=Formatted summary of similar results"`
	Results []SearchResult `json:"results" jsonschema:"description=List of similar content results"`
}

// === ANSWER INPUT/OUTPUT ===

// AnswerInput holds the parameters for an [Answer] call. Query is required;
// IncludeText controls whether full source text is included in the returned
// Citations.
type AnswerInput struct {
	Query       string `json:"query" jsonschema:"description=The question to answer,required"`
	IncludeText bool   `json:"include_text,omitempty" jsonschema:"description=Include full text from citation sources"`
}

// AnswerOutput contains the AI-generated answer and its supporting citations
// as returned by the Exa Answer endpoint.
type AnswerOutput struct {
	Query     string     `json:"query" jsonschema:"description=The original question"`
	Answer    string     `json:"answer" jsonschema:"description=AI-generated answer based on search results"`
	Citations []Citation `json:"citations" jsonschema:"description=Sources used to generate the answer"`
}

// Citation represents a single web source used to ground an [Answer] response.
// Text is only populated when AnswerInput.IncludeText is true.
type Citation struct {
	Title         string `json:"title" jsonschema:"description=Title of the citation source"`
	URL           string `json:"url" jsonschema:"description=URL of the citation source"`
	Author        string `json:"author,omitempty" jsonschema:"description=Author of the content"`
	PublishedDate string `json:"published_date,omitempty" jsonschema:"description=Publication date of the content"`
	Text          string `json:"text,omitempty" jsonschema:"description=Full text of the citation if requested"`
}

// === INTERNAL API RESPONSE TYPES ===

// exaSearchAPIResponse represents the raw API response from Exa Search
type exaSearchAPIResponse struct {
	Results            []exaSearchResultItem `json:"results"`
	ResolvedSearchType string                `json:"resolvedSearchType,omitempty"`
	RequestID          string                `json:"requestId,omitempty"`
	CostDollars        *exaCost              `json:"costDollars,omitempty"`
}

// exaSearchResultItem represents a single result from Exa Search API
type exaSearchResultItem struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	URL             string    `json:"url"`
	Score           float64   `json:"score,omitempty"`
	PublishedDate   string    `json:"publishedDate,omitempty"`
	Author          string    `json:"author,omitempty"`
	Text            string    `json:"text,omitempty"`
	Highlights      []string  `json:"highlights,omitempty"`
	HighlightScores []float64 `json:"highlightScores,omitempty"`
	Summary         string    `json:"summary,omitempty"`
}

// exaAnswerAPIResponse represents the raw API response from Exa Answer
type exaAnswerAPIResponse struct {
	Answer      string                `json:"answer"`
	Citations   []exaSearchResultItem `json:"citations,omitempty"`
	Results     []exaSearchResultItem `json:"results,omitempty"`
	RequestID   string                `json:"requestId,omitempty"`
	CostDollars *exaCost              `json:"costDollars,omitempty"`
}

// exaCost represents cost information from Exa API
type exaCost struct {
	Total float64 `json:"total"`
}

// exaAPIError represents an error response from Exa API.
// ErrorMessage is named to avoid shadowing Go's built-in error interface.
type exaAPIError struct {
	ErrorMessage string `json:"error,omitempty"`
	Message      string `json:"message,omitempty"`
}
