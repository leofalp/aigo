// Package client provides the orchestration layer between raw LLM provider calls
// and higher-level AI interaction patterns. It manages conversation state, tool
// registration, observability, cost tracking, and structured output in a single,
// immutable Client value.
//
// The primary entry point is [New], which accepts an [ai.Provider] and a set of
// functional options (e.g. [WithMemory], [WithTools], [WithSystemPrompt]).
// For type-safe structured responses, use [NewStructured] or [FromBaseClient].
package client
