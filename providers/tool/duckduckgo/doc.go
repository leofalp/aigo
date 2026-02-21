// Package duckduckgo provides a tool implementation that queries the DuckDuckGo
// Instant Answer API, enabling AI agents to perform web searches at no cost.
// It exposes two tools: a concise summary tool via [NewDuckDuckGoSearchTool] and
// a fully structured result tool via [NewDuckDuckGoSearchAdvancedTool].
// Both tools accept an [Input] query and communicate with the public,
// unauthenticated DuckDuckGo Instant Answer API endpoint.
package duckduckgo
