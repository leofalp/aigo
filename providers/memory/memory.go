package memory

import (
	"context"

	"github.com/leofalp/aigo/providers/ai"
)

// Provider defines memory operations for chat sessions and tool calls.
type Provider interface {
	// Core chat history
	AppendMessage(ctx context.Context, message *ai.Message)

	// Introspection
	Count() int
	AllMessages() []ai.Message
	LastMessages(n int) []ai.Message
	PopLastMessage() *ai.Message

	// clear
	ClearMessages(ctx context.Context)

	// Token-aware retrieval and compaction
	//GetForTokenBudget(maxTokens int, tokenizer ai.Tokenizer) []ai.Message
	//SummarizeOlder(maxTokens int, summarizer ai.Summarizer) error

	// Mutation by id
	//GetByID(id string) (*ai.Message, bool)
	//UpdateByID(id string, patch ai.MessagePatch) error
	//DeleteByID(id string) error

	// Truncation and eviction
	//TruncateLast(n int)
	//SetMaxMessages(n int)
	//SetTTLSeconds(ttl int64)

	// Metadata and filtering
	//SetMetadata(id string, meta map[string]string) error
	FilterByRole(role ai.MessageRole) []ai.Message

	// Tool calls
	//LogToolCall(tc ai.ToolCall)
	//GetToolCalls() []ai.ToolCall

	// Sessions and persistence
	//StartSession(id string) error
	//CurrentSession() string
	//ListSessions() []string
	//SwitchSession(id string) error
	//Save() error
	//Load() error

	// Semantic memory (optional)
	//UpsertEmbedding(id string, text string, vector []float32) error
	//SearchSimilar(query string, topK int) []ai.Message
}
