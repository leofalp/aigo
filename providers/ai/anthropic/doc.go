// Package anthropic implements the [ai.Provider] and [ai.StreamProvider] interfaces
// for Anthropic's Messages API.
//
// It handles request conversion from the generic [ai.ChatRequest] format to
// Anthropic's Messages wire format, response mapping back to [ai.ChatResponse],
// SSE-based streaming, and token cost calculation.
//
// The primary entry point is [New], which reads ANTHROPIC_API_KEY and
// ANTHROPIC_API_BASE_URL from the environment. Use [AnthropicProvider.WithAPIKey],
// [AnthropicProvider.WithBaseURL], or [AnthropicProvider.WithHttpClient] to configure
// the provider programmatically. Capabilities such as extended thinking, prompt
// caching, and vision are controlled via [AnthropicProvider.WithCapabilities].
package anthropic
