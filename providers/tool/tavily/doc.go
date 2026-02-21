// Package tavily provides tool implementations for the Tavily AI search and
// content extraction APIs, designed for LLM and RAG applications.
//
// It exposes three ready-to-use [tool.Tool] constructors: [NewTavilySearchTool]
// for summarised web search results, [NewTavilySearchAdvancedTool] for full
// structured search data, and [NewTavilyExtractTool] for parsing web page
// content into clean markdown.
//
// All tools require the TAVILY_API_KEY environment variable to be set.
package tavily
