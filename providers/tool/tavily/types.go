package tavily

// SearchInput represents the input parameters for Tavily Search.
// Query is required; all other fields are optional.
type SearchInput struct {
	Query          string   `json:"query" jsonschema:"description=The search query to perform,required"`
	SearchDepth    string   `json:"search_depth,omitempty" jsonschema:"description=Search depth: basic (faster/1 credit) or advanced (more thorough/2 credits),enum=basic,enum=advanced"`
	MaxResults     int      `json:"max_results,omitempty" jsonschema:"description=Number of results to return (default: 10),minimum=1,maximum=20"`
	IncludeDomains []string `json:"include_domains,omitempty" jsonschema:"description=List of domains to specifically include in search results"`
	ExcludeDomains []string `json:"exclude_domains,omitempty" jsonschema:"description=List of domains to exclude from search results"`
	IncludeAnswer  bool     `json:"include_answer,omitempty" jsonschema:"description=Include an AI-generated answer summary based on search results"`
	IncludeImages  bool     `json:"include_images,omitempty" jsonschema:"description=Include related images in the response"`
	Topic          string   `json:"topic,omitempty" jsonschema:"description=Search topic for optimization,enum=general,enum=news"`
}

// SearchOutput represents a simplified search result optimized for LLM consumption.
// It includes the original query, an optional AI-generated answer, a formatted summary, and structured results.
type SearchOutput struct {
	Query   string         `json:"query" jsonschema:"description=The original search query"`
	Answer  string         `json:"answer,omitempty" jsonschema:"description=AI-generated answer if include_answer was true"`
	Summary string         `json:"summary" jsonschema:"description=Formatted summary of search results"`
	Results []SearchResult `json:"results" jsonschema:"description=List of search results"`
}

// SearchResult represents a single search result from a Tavily search query.
type SearchResult struct {
	Title   string  `json:"title" jsonschema:"description=Title of the result"`
	URL     string  `json:"url" jsonschema:"description=URL of the result"`
	Content string  `json:"content" jsonschema:"description=Content snippet from the result"`
	Score   float64 `json:"score,omitempty" jsonschema:"description=Relevance score of the result"`
}

// SearchAdvancedOutput represents the complete Tavily Search API response with all metadata.
// It includes results with full content, images, and API timing information.
type SearchAdvancedOutput struct {
	Query        string                 `json:"query" jsonschema:"description=The original search query"`
	Answer       string                 `json:"answer,omitempty" jsonschema:"description=AI-generated answer if requested"`
	Results      []SearchResultAdvanced `json:"results" jsonschema:"description=List of detailed search results"`
	Images       []ImageResult          `json:"images,omitempty" jsonschema:"description=Related images if requested"`
	ResponseTime float64                `json:"response_time" jsonschema:"description=API response time in seconds"`
	RequestID    string                 `json:"request_id,omitempty" jsonschema:"description=Unique request identifier"`
}

// SearchResultAdvanced represents a detailed search result with all metadata.
// It includes raw page content, images found on the page, and relevance scoring.
type SearchResultAdvanced struct {
	Title      string   `json:"title" jsonschema:"description=Title of the result"`
	URL        string   `json:"url" jsonschema:"description=URL of the result"`
	Content    string   `json:"content" jsonschema:"description=Content snippet from the result"`
	RawContent string   `json:"raw_content,omitempty" jsonschema:"description=Full raw content of the page if available"`
	Score      float64  `json:"score" jsonschema:"description=Relevance score of the result"`
	Images     []string `json:"images,omitempty" jsonschema:"description=Images found on the page"`
}

// ImageResult represents an image from search results.
type ImageResult struct {
	URL         string `json:"url" jsonschema:"description=URL of the image"`
	Description string `json:"description,omitempty" jsonschema:"description=Description of the image"`
}

// ExtractInput represents the input parameters for Tavily Extract.
// URLs is required; ExtractDepth is optional.
type ExtractInput struct {
	URLs         []string `json:"urls" jsonschema:"description=URLs to extract content from (max 20),required"`
	ExtractDepth string   `json:"extract_depth,omitempty" jsonschema:"description=Extraction depth: basic (1 credit per 5 URLs) or advanced (2 credits per 5 URLs),enum=basic,enum=advanced"`
}

// ExtractOutput represents the simplified extract result containing parsed web page content.
type ExtractOutput struct {
	Results []ExtractResult `json:"results" jsonschema:"description=List of extracted content"`
	Summary string          `json:"summary" jsonschema:"description=Summary of extracted content"`
}

// ExtractResult represents extracted content from a single URL in markdown format.
type ExtractResult struct {
	URL        string `json:"url" jsonschema:"description=The URL that was extracted"`
	RawContent string `json:"raw_content" jsonschema:"description=Extracted content in markdown format"`
	Favicon    string `json:"favicon,omitempty" jsonschema:"description=Favicon URL of the site"`
}

// === Internal API Response Types ===

// tavilySearchAPIResponse represents the raw API response from Tavily Search
type tavilySearchAPIResponse struct {
	Query        string                   `json:"query"`
	Answer       string                   `json:"answer,omitempty"`
	Results      []tavilySearchResultItem `json:"results"`
	Images       []tavilyImageItem        `json:"images,omitempty"`
	ResponseTime float64                  `json:"response_time"`
	RequestID    string                   `json:"request_id"`
}

// tavilySearchResultItem represents a single result from Tavily Search API
type tavilySearchResultItem struct {
	Title      string   `json:"title"`
	URL        string   `json:"url"`
	Content    string   `json:"content"`
	RawContent string   `json:"raw_content,omitempty"`
	Score      float64  `json:"score"`
	Images     []string `json:"images,omitempty"`
}

// tavilyImageItem represents an image from Tavily Search API
type tavilyImageItem struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// tavilyExtractAPIResponse represents the raw API response from Tavily Extract
type tavilyExtractAPIResponse struct {
	Results       []tavilyExtractResultItem `json:"results"`
	FailedResults []tavilyFailedResult      `json:"failed_results,omitempty"`
	ResponseTime  float64                   `json:"response_time"`
	RequestID     string                    `json:"request_id"`
}

// tavilyExtractResultItem represents a single result from Tavily Extract API
type tavilyExtractResultItem struct {
	URL        string   `json:"url"`
	RawContent string   `json:"raw_content"`
	Images     []string `json:"images,omitempty"`
	Favicon    string   `json:"favicon,omitempty"`
}

// tavilyFailedResult represents a URL that failed to be extracted
type tavilyFailedResult struct {
	URL   string `json:"url"`
	Error string `json:"error,omitempty"`
}

// tavilyAPIError represents an error response from Tavily API
type tavilyAPIError struct {
	Detail struct {
		Error string `json:"error"`
	} `json:"detail"`
}
