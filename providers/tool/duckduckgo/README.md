# DuckDuckGo Search Tool

A tool for performing web searches using the DuckDuckGo Instant Answer API.

## Features

- Free web search without API key requirement
- Returns instant answers, abstracts, definitions, and related topics
- Simple integration with the aigo framework
- Support for context cancellation
- Automatic conversion of relative URLs to absolute URLs

## Two Available Versions

### Base Version (`DuckDuckGoSearch`)
Returns a text summary of results, ideal for quick and concise answers.

### Advanced Version (`DuckDuckGoSearchAdvanced`)
Returns the complete structured response from the DuckDuckGo API with all metadata, images, and related topics. Ideal when you need access to all details.

## Usage

See the complete example in `examples/duckduckgoExample/main.go` which shows 4 use cases:

1. **Direct Use - Base**: Search with text summary
2. **Direct Use - Advanced**: Search with complete structured output
3. **AI Integration - Base**: Base tool integrated with AI client
4. **AI Integration - Advanced**: Advanced tool integrated with AI client

### Quick Start - Base (Summary)

```go
input := duckduckgo.Input{Query: "Go programming language"}
output, err := duckduckgo.Search(context.Background(), input)
// output.Summary contains a text summary
```

### Quick Start - Advanced (Structured)

```go
input := duckduckgo.Input{Query: "Albert Einstein"}
output, err := duckduckgo.SearchAdvanced(context.Background(), input)
// output contains Abstract, Image, RelatedTopics, etc.
// All relative URLs are converted to absolute URLs
```

### With AI Client

```go
client := client.NewClient[string](provider).
    AddTools([]tool.GenericTool{
        duckduckgo.NewDuckDuckGoSearchTool(),        // Base
        // or
        duckduckgo.NewDuckDuckGoSearchAdvancedTool(), // Advanced
    })
```

## Input and Output

### Input

```go
type Input struct {
	Query string `json:"query"` // The search query
}
```

### Output (Base)

```go
type Output struct {
	Query   string `json:"query"`   // The original query
	Summary string `json:"summary"` // Summary of search results
}
```

The `Summary` may contain:
- **Abstract**: Detailed information about the topic
- **Answer**: Instant answers (e.g., calculations, conversions)
- **Definition**: Dictionary definitions
- **Related Topics**: Related topics (max 5)
- **Source**: URL of the information source

### Output (Advanced)

```go
type AdvancedOutput struct {
	Query          string         // The original query
	Abstract       string         // Abstract text about the topic
	AbstractSource string         // Source name (e.g., Wikipedia)
	AbstractURL    string         // URL of the source
	Answer         string         // Instant answer
	AnswerType     string         // Type of answer
	Definition     string         // Dictionary definition
	DefinitionURL  string         // URL of the definition
	Heading        string         // Result heading
	Image          string         // Relevant image URL (absolute)
	ImageWidth     string         // Image width
	ImageHeight    string         // Image height
	ImageIsLogo    string         // "1" if it's a logo
	RelatedTopics  []RelatedTopic // Complete list of related topics
	Results        []Result       // Additional results
	Type           string         // Type (A=article, C=category, D=disambiguation, E=exclusive, N=nothing)
	Redirect       string         // Redirect URL if applicable
}

type RelatedTopic struct {
	FirstURL string // Topic URL
	Icon     Icon   // Associated icon (with absolute URL)
	Result   string // HTML result
	Text     string // Topic text
}
```

The advanced version provides:
- **Complete structure** from the DuckDuckGo API
- **Image metadata** with dimensions
- **All related topics** without limits
- **Icons and URLs** for each topic (all URLs are absolute)
- **Result type** for classification

## Notes

- Uses the DuckDuckGo Instant Answer API which is free and requires no authentication
- The API may not return results for all queries
- **Base Version**: Limits related topics to 5 to avoid overly long output
- **Advanced Version**: Returns all topics without limits and converts all URLs to absolute
- The tool automatically handles context for request cancellation
- Automatically handles both numeric and string values for image dimensions
- All icon URLs in the advanced version are converted to absolute URLs

## Architecture

The code has been optimized to eliminate duplication:
- **`fetchDDGResponse()`**: Shared function for API calls
- **`Search()`**: Wrapper that returns a text summary
- **`SearchAdvanced()`**: Wrapper that returns complete structured output
- **`flexibleInt`**: Private type that automatically handles int/string values
- **`makeAbsoluteURL()`**: Converts relative URLs to absolute URLs

## Testing

Run tests with:

```bash
go test ./providers/tool/duckduckgo/
```

For verbose test output:

```bash
go test -v ./providers/tool/duckduckgo/
```

Quick test without AI:

```bash
go run examples/duckduckgoExample/test_direct.go
```

