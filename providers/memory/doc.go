// Package memory defines the Provider interface for conversation history management.
// Implementations are responsible for storing, retrieving, and filtering [ai.Message]
// values across a chat session. The interface is intentionally minimal: it covers
// the operations required by the core client for turn-based conversations.
// Read methods return errors so that database-backed implementations can
// surface failures instead of silently swallowing them.
// The bundled reference implementation lives in the sibling package
// [github.com/leofalp/aigo/providers/memory/inmemory].
package memory
