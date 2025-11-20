# Brave Search Tool

A powerful web search tool for aigo that uses the Brave Search API to provide high-quality, privacy-focused search results.

## 🌟 Why Brave Search?

- ✅ **Real Web Search** - Not limited to instant answers like DuckDuckGo
- ✅ **High Quality Results** - Independent index with 30+ billion pages
- ✅ **Privacy-Focused** - No tracking, no user profiling
- ✅ **Generous Free Tier** - 2,000 queries/month for free
- ✅ **Fresh Content** - 100M+ page updates daily
- ✅ **Rich Results** - Web, news, videos, infoboxes, and more
- ✅ **LLM-Friendly** - Perfect for AI agents and ReAct patterns

## 📋 Prerequisites

You need a Brave Search API key:

1. Visit [Brave Search API](https://brave.com/search/api/)
2. Sign up for a free account (requires credit card for verification, but won't charge on free tier)
3. Get your API key
4. Set it as an environment variable:

```bash
export BRAVE_SEARCH_API_KEY="your_api_key_here"
```

Or add it to your `.env` file:

```env
BRAVE_SEARCH_API_KEY=your_api_key_here
```

## 🚀 Features

### Two Versions Available

#### 1. **BraveSearch** (Basic - Recommended for LLMs)
Returns a summarized, formatted text output optimized for LLM consumption.

**Best for:**
- ReAct patterns and AI agents
- Quick information retrieval
- When you need concise, readable summaries

#### 2. **BraveSearchAdvanced** (Complete Data)
Returns the full structured API response with all metadata.

**Best for:**
- When you need all available data
- Building custom result presentations
- Extracting specific metadata fields

### Supported Search Features

- **Web Search** - General web results with descriptions
- **News Search** - Recent news articles with timestamps
- **Video Search** - Video results from multiple platforms
- **Infoboxes** - Structured entity information (people, places, things)
- **Location Results** - Map and local business results
- **Freshness Filters** - Search by time (past day, week, month, year)
- **Localization** - Country and language-specific results
- **Safe Search** - Content filtering options

## 📖 Usage

### Quick Start - Basic Search

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/leofalp/aigo/providers/tool/bravesearch"
)

func main() {
	input := bravesearch.Input{
		Query: "Go programming language tutorial",
		Count: 10,
	}

	output, err := bravesearch.Search(context.Background(), input)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Summary:")
	fmt.Println(output.Summary)
	fmt.Printf("\nFound %d results\n", len(output.Results))
}
```

### Advanced Search with Options

```go
input := bravesearch.Input{
	Query:      "best Italian restaurants",
	Count:      10,
	Country:    "us",           // Localize to US
	SearchLang: "en",           // English results
	SafeSearch: "moderate",     // Filter adult content
	Freshness:  "pw",           // Past week only
}

output, err := bravesearch.Search(context.Background(), input)
```

### Using with AI Client

```go
package main

import (
	"context"
	"log"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/providers/ai/openai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
	"github.com/leofalp/aigo/providers/tool/bravesearch"
)

func main() {
	// Create Brave Search tool
	searchTool := bravesearch.NewBraveSearchTool()

	// Create AI client with tool
	aiClient, err := client.New(
		openai.New(),
		client.WithMemory(inmemory.New()),
		client.WithTools(searchTool),
		client.WithEnrichSystemPromptWithToolsDescriptions(),
	)
	if err != nil {
		log.Fatal(err)
	}

	// The LLM can now use web search!
	response, err := aiClient.SendMessage(
		context.Background(),
		"What are the latest developments in quantum computing?",
	)
	if err != nil {
		log.Fatal(err)
	}

	println(response.Content)
}
```

### Using with ReAct Pattern

```go
package main

import (
	"context"
	"log"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/patterns/react"
	"github.com/leofalp/aigo/providers/ai/openai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
	"github.com/leofalp/aigo/providers/tool/bravesearch"
	"github.com/leofalp/aigo/providers/tool/calculator"
)

func main() {
	// Create tools
	searchTool := bravesearch.NewBraveSearchTool()
	calcTool := calculator.NewCalculatorTool()

	// Create base client
	baseClient, err := client.New(
		openai.New(),
		client.WithMemory(inmemory.New()),
		client.WithTools(searchTool, calcTool),
		client.WithEnrichSystemPromptWithToolsDescriptions(),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create ReAct pattern
	reactPattern, err := react.NewReactPattern(
		baseClient,
		react.WithMaxIterations(5),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Execute with web search capability
	result, err := reactPattern.Execute(
		context.Background(),
		"Search for recent news about AI and summarize the top 3 findings",
	)
	if err != nil {
		log.Fatal(err)
	}

	println(result.LastResponse.Content)
}
```

## 📝 Input Parameters

### Input Struct

```go
type Input struct {
	Query      string  // Required: The search query
	Count      int     // Optional: Number of results (1-20, default: 10)
	Country    string  // Optional: Country code (e.g., "us", "uk", "de")
	SearchLang string  // Optional: Language code (e.g., "en", "es", "fr")
	SafeSearch string  // Optional: "off", "moderate", "strict" (default: "moderate")
	Freshness  string  // Optional: "pd" (day), "pw" (week), "pm" (month), "py" (year)
}
```

### Parameters Details

#### Query (required)
The search query string. Works with:
- Natural language questions: `"What is quantum computing?"`
- Keywords: `"Go programming tutorial"`
- Complex queries: `"best practices for microservices architecture"`
- Current events: `"latest technology news"`

#### Count (optional, default: 10)
Number of results to return. Range: 1-20.

#### Country (optional)
Two-letter country code for localized results:
- `"us"` - United States
- `"uk"` - United Kingdom
- `"de"` - Germany
- `"fr"` - France
- `"jp"` - Japan
- etc.

#### SearchLang (optional)
Two-letter language code:
- `"en"` - English
- `"es"` - Spanish
- `"fr"` - French
- `"de"` - German
- `"ja"` - Japanese
- etc.

#### SafeSearch (optional, default: "moderate")
- `"off"` - No filtering
- `"moderate"` - Filter adult content
- `"strict"` - Strict filtering

#### Freshness (optional)
Time-based filtering:
- `"pd"` - Past day (last 24 hours)
- `"pw"` - Past week
- `"pm"` - Past month
- `"py"` - Past year

## 📤 Output Formats

### Basic Output (BraveSearch)

```go
type Output struct {
	Query   string         // The original search query
	Summary string         // Formatted text summary of results
	Results []SearchResult // List of search results
}

type SearchResult struct {
	Title       string // Title of the result
	URL         string // URL of the result
	Description string // Description/snippet
	Age         string // Age of content (e.g., "2 hours ago")
}
```

**Example Summary:**
```
Found 10 web results:

1. Go Programming Language - Official Website
   URL: https://go.dev
   Go is an open source programming language that makes it simple to build secure, scalable systems...

2. Learn Go - Interactive Tutorial
   URL: https://tour.golang.org
   A Tour of Go is an interactive introduction to Go: the basic syntax and data structures...

[etc.]

Infobox: Go (programming language)
Description: Go is a statically typed, compiled high-level programming language designed at Google...
```

### Advanced Output (BraveSearchAdvanced)

```go
type AdvancedOutput struct {
	Query        string              // The original query
	Type         string              // Response type
	Web          *WebResults         // Web search results
	News         *NewsResults        // News articles
	Videos       *VideoResults       // Video results
	Infobox      *Infobox            // Entity information
	Locations    *LocationResults    // Location/map results
	MixedResults *MixedResultSection // Mixed content
}
```

See the code for complete type definitions of nested structures.

## 🎯 Best Practices

### For LLM/AI Applications

1. **Use the basic `BraveSearch` tool** - It provides pre-formatted summaries that LLMs can easily understand
2. **Set appropriate count** - Usually 5-10 results are sufficient for LLM context
3. **Use freshness filters** - For time-sensitive queries, use `Freshness: "pd"` or `"pw"`
4. **Localize when relevant** - Set `Country` and `SearchLang` for location-specific queries

### Query Optimization

✅ **Good queries:**
- `"latest developments in quantum computing"`
- `"best practices for Go microservices"`
- `"current AI news"`
- `"how to implement JWT authentication"`

❌ **Avoid:**
- Very broad queries: `"technology"` (too generic)
- Single words without context: `"Go"` (ambiguous)

### Rate Limiting

Free tier: 2,000 queries/month, 1 query/second

**Tips:**
- Cache results when possible
- Implement retry logic with exponential backoff
- Monitor your usage via Brave dashboard

## 🔍 Comparison with Other Tools

| Feature | Brave Search | DuckDuckGo | Google Custom | Bing API |
|---------|--------------|------------|---------------|----------|
| **Real web search** | ✅ | ❌ (instant answers only) | ✅ | ✅ |
| **Free tier** | 2,000/month | Unlimited | 100/day | 1,000/month |
| **Independent index** | ✅ | ✅ | ✅ | ✅ |
| **Privacy-focused** | ✅ | ✅ | ❌ | ❌ |
| **Complex queries** | ✅ | ❌ | ✅ | ✅ |
| **Fresh content** | ✅ | Limited | ✅ | ✅ |
| **API quality** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Cost (after free)** | $3-5/1k | N/A | $5/1k | $7/1k |

**Verdict:** Brave Search offers the best balance of quality, privacy, and generous free tier for AI applications.

## 🧪 Testing

Run tests with your API key:

```bash
# Set API key
export BRAVE_SEARCH_API_KEY="your_key"

# Run all tests
go test -v ./providers/tool/bravesearch/

# Run specific test
go test -v ./providers/tool/bravesearch/ -run TestSearch
```

Tests will automatically skip if `BRAVE_SEARCH_API_KEY` is not set.

## 🐛 Error Handling

### Common Errors

#### Missing API Key
```
Error: BRAVE_SEARCH_API_KEY environment variable is not set
```
**Solution:** Set the environment variable with your API key

#### Rate Limit Exceeded
```
Error: unexpected status code 429
```
**Solution:** You've exceeded your rate limit. Wait or upgrade your plan.

#### Invalid API Key
```
Error: unexpected status code 401
```
**Solution:** Check that your API key is correct

#### Network Errors
```
Error: error making request: ...
```
**Solution:** Check your internet connection and Brave API status

#### Gzip/Parsing Error
```
Error: error parsing response: invalid character '\x1f' looking for beginning of value
```
**Solution:** This is a gzip compression issue. The tool now handles this automatically. If you still see this error, make sure you're using the latest version of the tool. The http.Client automatically decompresses gzip responses when Accept-Encoding is not manually set.

## 📊 Pricing

| Plan | Queries/Month | Cost | Best For |
|------|---------------|------|----------|
| **Free AI** | 2,000 | $0 | Development, testing, small projects |
| **Base AI** | Pay-as-you-go | $5/1,000 queries | Medium projects |
| **Pro AI** | Unlimited | $9/1,000 queries | Production, high-volume |
| **Enterprise** | Custom | Custom pricing | Large-scale applications |

**Note:** Free tier requires credit card for verification but won't charge.

## 🔗 Resources

- [Brave Search API Documentation](https://brave.com/search/api/)
- [API Playground](https://brave.com/search/api/playground)
- [Pricing Details](https://brave.com/search/api/pricing)
- [Support](https://community.brave.com/)

## 📄 License

This tool is part of the aigo framework. See the main project license for details.

## 🤝 Contributing

Contributions are welcome! Please see the main aigo project for contribution guidelines.

## ⚠️ Important Notes

1. **API Key Security**: Never commit your API key to version control. Use environment variables.
2. **Rate Limits**: Respect the rate limits to avoid service interruption.
3. **Terms of Service**: Review Brave's terms of service for commercial usage.
4. **Data Rights**: Free tier doesn't grant data storage rights. Upgrade if you need to store results.
5. **Automatic Gzip Handling**: The tool automatically handles gzip compression from the API. Go's http.Client transparently decompresses responses.

## 💡 Tips for AI Agents

When using with ReAct or other agent patterns:

1. **Let the LLM decide when to search** - Don't force every query to use search
2. **Provide context** - The tool description helps the LLM understand when to use it
3. **Handle no results gracefully** - The tool returns helpful messages when no results are found
4. **Combine with other tools** - Search works great alongside calculator, weather, etc.
5. **Use freshness for time-sensitive queries** - LLMs can request recent results when needed

## 🎓 Examples

See the `examples/` directory for complete working examples:
- Basic search usage
- Integration with AI client
- ReAct pattern with web search
- Multi-tool agent with search capability