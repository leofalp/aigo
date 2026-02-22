// Package openai implements the aigo AI provider interface for OpenAI-compatible APIs.
// It supports both the /v1/responses endpoint (native OpenAI only) and the universal
// /v1/chat/completions endpoint, selecting the best option automatically based on
// detected provider capabilities.
//
// The main entry point is [New], which reads OPENAI_API_KEY and OPENAI_API_BASE_URL
// from the environment and auto-detects capabilities for well-known hosts (OpenAI,
// Azure, Ollama, OpenRouter). Use [OpenAIProvider.WithAPIKey] and
// [OpenAIProvider.WithBaseURL] to override these values programmatically.
//
// Streaming is available through [OpenAIProvider.StreamMessage], which returns an
// [ai.ChatStream] iterator over incremental SSE events.
package openai
