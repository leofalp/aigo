package memory

import (
	"context"

	"github.com/leofalp/aigo/providers/ai"
)

// Provider defines the contract for conversation-history storage.
// Implementations must be safe for concurrent use; the core client calls
// these methods from multiple goroutines during tool-call loops.
//
// All methods accept context.Context for proper cancellation, timeout,
// and tracing propagation. In-memory implementations may ignore the context.
//
// Read methods return error to allow database-backed implementations to
// surface query failures instead of swallowing them silently.
//
// The reference implementation is [github.com/leofalp/aigo/providers/memory/inmemory.ArrayMemory].
type Provider interface {
	// AppendMessage adds message to the end of the conversation history.
	// Implementations should store a copy so callers can reuse the pointer safely.
	// A nil message must be silently ignored.
	AppendMessage(ctx context.Context, message *ai.Message)

	// Count returns the total number of messages currently stored.
	Count(ctx context.Context) (int, error)

	// AllMessages returns a copy of every message in chronological order.
	// Callers may freely modify the returned slice without affecting stored state.
	AllMessages(ctx context.Context) ([]ai.Message, error)

	// LastMessages returns a copy of the most recent n messages in chronological order.
	// If n exceeds the total number of stored messages, all messages are returned.
	// If n is zero or negative, an empty slice is returned.
	LastMessages(ctx context.Context, n int) ([]ai.Message, error)

	// PopLastMessage removes and returns the most recent message.
	// Returns nil when the history is empty.
	PopLastMessage(ctx context.Context) (*ai.Message, error)

	// ClearMessages removes all stored messages.
	// Implementations should retain any underlying capacity for reuse.
	ClearMessages(ctx context.Context)

	// FilterByRole returns a copy of all messages whose role matches role.
	// Returns an empty slice when no matching messages exist.
	FilterByRole(ctx context.Context, role ai.MessageRole) ([]ai.Message, error)

	// Token-aware retrieval and compaction (planned)
	//GetForTokenBudget(maxTokens int, tokenizer ai.Tokenizer) []ai.Message
	//SummarizeOlder(maxTokens int, summarizer ai.Summarizer) error

	// Mutation by id (planned)
	//GetByID(id string) (*ai.Message, bool)
	//UpdateByID(id string, patch ai.MessagePatch) error
	//DeleteByID(id string) error

	// Truncation and eviction (planned)
	//TruncateLast(n int)
	//SetMaxMessages(n int)
	//SetTTLSeconds(ttl int64)

	// Metadata and filtering (planned)
	//SetMetadata(id string, meta map[string]string) error

	// Tool calls (planned)
	//LogToolCall(tc ai.ToolCall)
	//GetToolCalls() []ai.ToolCall

	// Sessions and persistence (planned)
	//StartSession(id string) error
	//CurrentSession() string
	//ListSessions() []string
	//SwitchSession(id string) error
	//Save() error
	//Load() error

	// Semantic memory (planned)
	//UpsertEmbedding(id string, text string, vector []float32) error
	//SearchSimilar(query string, topK int) []ai.Message
}
