// Package webfetch provides an aigo tool that fetches web pages over HTTP/HTTPS
// and converts their HTML content into Markdown for consumption by language models.
//
// The main entry point is [NewWebFetchTool], which returns a ready-to-use
// [tool.Tool] that can be registered with an aigo client via client.WithTools.
// The underlying fetch logic is also available directly through the [Fetch] function.
//
// URL normalisation, redirect following, response-size limiting, and
// context-aware cancellation are handled automatically.
package webfetch
