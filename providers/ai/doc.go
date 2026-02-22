// Package ai defines the shared, provider-agnostic types and interfaces used
// across all LLM provider implementations (OpenAI, Gemini, Anthropic, etc.).
// Each provider's conversion layer is responsible for mapping these types to
// its own wire format, keeping the rest of the codebase decoupled from
// provider-specific details.
//
// The two central interfaces are [Provider] for synchronous chat completions
// and [StreamProvider] for SSE-based streaming responses. Request data flows
// through [ChatRequest] and responses are returned as [ChatResponse].
// For real-time streaming, [ChatStream] and [StreamEvent] carry incremental
// deltas to the caller.
package ai
