package exa

// === SEARCH INPUT/OUTPUT ===

// SearchInput represents the input parameters for Exa Search
type SearchInput struct {
	Query              string   `json:"query" jsonschema:"description=The search query to perform,required"`
	Type               string   `json:"type,omitempty" jsonschema:"description=Search type: neural (embedding-based) auto (default) fast (optimized for speed) or deep (comprehensive),enum=neural,enum=auto,enum=fast,enum=deep"`
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

// SearchOutput represents a simplified search result optimized for LLM consumption
type SearchOutput struct {
	Query   string         `json:"query" jsonschema:"description=The original search query"`
	Summary string         `json:"summary" jsonschema:"description=Formatted summary of search results"`
	Results []SearchResult `json:"results" jsonschema:"description=List of search results"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Title         string   `json:"title" jsonschema:"description=Title of the result"`
	URL           string   `json:"url" jsonschema:"description=URL of the result"`
	PublishedDate string   `json:"published_date,omitempty" jsonschema:"description=Publication date of the content"`
	Author        string   `json:"author,omitempty" jsonschema:"description=Author of the content"`
	Text          string   `json:"text,omitempty" jsonschema:"description=Full text content if requested"`
	Highlights    []string `json:"highlights,omitempty" jsonschema:"description=Key sentence highlights if requested"`
}

// SearchAdvancedOutput represents the complete Exa Search API response
type SearchAdvancedOutput struct {
	Query              string                 `json:"query" jsonschema:"description=The original search query"`
	Results            []SearchResultAdvanced `json:"results" jsonschema:"description=List of detailed search results"`
	ResolvedSearchType string                 `json:"resolved_search_type,omitempty" jsonschema:"description=The actual search type used"`
	RequestID          string                 `json:"request_id,omitempty" jsonschema:"description=Unique request identifier"`
}

// SearchResultAdvanced represents a detailed search result with all metadata
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

// SimilarInput represents the input parameters for Exa FindSimilar
type SimilarInput struct {
	URL               string   `json:"url,omitempty" jsonschema:"description=URL to find similar content for (provide either url or text)"`
	Text              string   `json:"text,omitempty" jsonschema:"description=Text to find similar content for (provide either url or text)"`
	NumResults        int      `json:"num_results,omitempty" jsonschema:"description=Number of results to return (default: 10 max: 100),minimum=1,maximum=100"`
	IncludeDomains    []string `json:"include_domains,omitempty" jsonschema:"description=List of domains to include in results"`
	ExcludeDomains    []string `json:"exclude_domains,omitempty" jsonschema:"description=List of domains to exclude from results"`
	IncludeText       bool     `json:"include_text,omitempty" jsonschema:"description=Include full page text content in results"`
	IncludeHighlights bool     `json:"include_highlights,omitempty" jsonschema:"description=Include key sentence highlights in results"`
}

// SimilarOutput represents the FindSimilar result
type SimilarOutput struct {
	SourceURL string         `json:"source_url,omitempty" jsonschema:"description=The source URL used for similarity search"`
	Summary   string         `json:"summary" jsonschema:"description=Formatted summary of similar results"`
	Results   []SearchResult `json:"results" jsonschema:"description=List of similar content results"`
}

// === ANSWER INPUT/OUTPUT ===

// AnswerInput represents the input parameters for Exa Answer
type AnswerInput struct {
	Query       string `json:"query" jsonschema:"description=The question to answer,required"`
	IncludeText bool   `json:"include_text,omitempty" jsonschema:"description=Include full text from citation sources"`
}

// AnswerOutput represents the Answer result
type AnswerOutput struct {
	Query     string     `json:"query" jsonschema:"description=The original question"`
	Answer    string     `json:"answer" jsonschema:"description=AI-generated answer based on search results"`
	Citations []Citation `json:"citations" jsonschema:"description=Sources used to generate the answer"`
}

// Citation represents a source citation for an answer
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

// exaAPIError represents an error response from Exa API
type exaAPIError struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}
