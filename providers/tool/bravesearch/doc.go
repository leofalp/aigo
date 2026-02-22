// Package bravesearch provides a tool adapter for the Brave Search API,
// enabling LLM agents to perform privacy-focused web searches.
// It exposes two entry points: [NewBraveSearchTool] for summarized results
// optimized for LLM consumption, and [NewBraveSearchAdvancedTool] for the
// complete structured API response including news, videos, infoboxes, and
// location data. Both tools require the BRAVE_SEARCH_API_KEY environment
// variable to be set.
package bravesearch
