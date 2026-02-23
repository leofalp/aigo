package inmemory

import (
	"context"
	"sync"

	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/memory"
	"github.com/leofalp/aigo/providers/observability"
)

// ArrayMemory is a simple, concurrency-safe in-memory message store.
// It uses RWMutex to guard access and is efficient for read-heavy workloads.
type ArrayMemory struct {
	mu       sync.RWMutex
	messages []ai.Message
}

// New returns a new, empty [ArrayMemory] ready for immediate use.
// The internal message slice is pre-allocated to avoid extra allocations on the first appends.
func New() *ArrayMemory {
	return &ArrayMemory{
		messages: []ai.Message{},
	}
}

// Ensure ArrayMemory implements memory.Provider at compile time.
var _ memory.Provider = (*ArrayMemory)(nil)

// AppendMessage stores a copy of message at the end of the history.
// It is a no-op when message is nil.
// When an observability span is present in ctx, an event is recorded with the
// message role and content length, and the running total message count is set
// as a span attribute so callers can track history growth through tracing.
func (m *ArrayMemory) AppendMessage(ctx context.Context, message *ai.Message) {
	if message == nil {
		return
	}

	span := observability.SpanFromContext(ctx)

	if span != nil {
		span.AddEvent(observability.EventMemoryAppend,
			observability.String(observability.AttrMemoryMessageRole, string(message.Role)),
			observability.Int(observability.AttrMemoryMessageLength, len(message.Content)),
		)
	}

	m.mu.Lock()
	m.messages = append(m.messages, *message)
	totalMessages := len(m.messages)
	m.mu.Unlock()

	if span != nil {
		span.SetAttributes(
			observability.Int(observability.AttrMemoryTotalMessages, totalMessages),
		)
	}
}

// Count returns the number of messages stored.
// The context parameter is accepted for interface compliance but is not used
// by the in-memory implementation. The returned error is always nil.
func (m *ArrayMemory) Count(_ context.Context) (int, error) {
	m.mu.RLock()
	n := len(m.messages)
	m.mu.RUnlock()
	return n, nil
}

// AllMessages returns a copy of all messages to avoid external mutation of internal state.
// The context parameter is accepted for interface compliance but is not used
// by the in-memory implementation. The returned error is always nil.
func (m *ArrayMemory) AllMessages(_ context.Context) ([]ai.Message, error) {
	m.mu.RLock()
	if len(m.messages) == 0 {
		m.mu.RUnlock()
		return []ai.Message{}, nil
	}
	out := make([]ai.Message, len(m.messages))
	copy(out, m.messages)
	m.mu.RUnlock()
	return out, nil
}

// LastMessages returns up to the last count messages as a new, independent slice.
// If count exceeds the total number of stored messages, all messages are returned.
// Returns an empty, non-nil slice when count is zero or negative, or when the store is empty.
// The context parameter is accepted for interface compliance but is not used
// by the in-memory implementation. The returned error is always nil.
func (m *ArrayMemory) LastMessages(_ context.Context, n int) ([]ai.Message, error) {
	if n <= 0 {
		return []ai.Message{}, nil
	}
	m.mu.RLock()
	if len(m.messages) == 0 {
		m.mu.RUnlock()
		return []ai.Message{}, nil
	}
	if n > len(m.messages) {
		n = len(m.messages)
	}
	start := len(m.messages) - n
	out := make([]ai.Message, n)
	copy(out, m.messages[start:])
	m.mu.RUnlock()
	return out, nil
}

// PopLastMessage removes and returns the last message, or nil if empty.
// The context parameter is accepted for interface compliance but is not used
// by the in-memory implementation. The returned error is always nil.
func (m *ArrayMemory) PopLastMessage(_ context.Context) (*ai.Message, error) {
	m.mu.Lock()
	if len(m.messages) == 0 {
		m.mu.Unlock()
		return nil, nil
	}
	idx := len(m.messages) - 1
	msg := m.messages[idx]
	m.messages = m.messages[:idx]
	m.mu.Unlock()
	return &msg, nil
}

// ClearMessages removes all messages while retaining the underlying slice capacity,
// so subsequent appends do not immediately trigger a reallocation.
// When an observability span is present in ctx, a clear event is recorded before
// the store is reset.
func (m *ArrayMemory) ClearMessages(ctx context.Context) {
	span := observability.SpanFromContext(ctx)

	if span != nil {
		span.AddEvent(observability.EventMemoryClear)
	}

	m.mu.Lock()
	m.messages = m.messages[:0]
	m.mu.Unlock()
}

// FilterByRole returns a copy of all messages whose role matches the given role.
// The returned slice is always non-nil; an empty slice is returned when no messages match.
// The context parameter is accepted for interface compliance but is not used
// by the in-memory implementation. The returned error is always nil.
func (m *ArrayMemory) FilterByRole(_ context.Context, role ai.MessageRole) ([]ai.Message, error) {
	m.mu.RLock()
	if len(m.messages) == 0 {
		m.mu.RUnlock()
		return []ai.Message{}, nil
	}
	filtered := make([]ai.Message, 0, len(m.messages))
	for _, msg := range m.messages {
		if msg.Role == role {
			filtered = append(filtered, msg)
		}
	}
	m.mu.RUnlock()
	if len(filtered) == 0 {
		return []ai.Message{}, nil
	}
	out := make([]ai.Message, len(filtered))
	copy(out, filtered)
	return out, nil
}
