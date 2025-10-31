package inmemory

import (
	"aigo/providers/ai"
	"aigo/providers/memory"
	"sync"
)

// ArrayMemory is a simple, concurrency-safe in-memory message store.
// It uses RWMutex to guard access and is efficient for read-heavy workloads.
type ArrayMemory struct {
	mu       sync.RWMutex
	messages []ai.Message
}

func NewArrayMemory() *ArrayMemory {
	return &ArrayMemory{
		messages: []ai.Message{},
	}
}

// Ensure ArrayMemory implements memory.Provider
var _ memory.Provider = (*ArrayMemory)(nil)

// AppendMessage stores a copy of the provided message at the end of the history.
func (m *ArrayMemory) AppendMessage(message *ai.Message) {
	if message == nil {
		return
	}
	m.mu.Lock()
	m.messages = append(m.messages, *message)
	m.mu.Unlock()
}

// Count returns the number of messages stored.
func (m *ArrayMemory) Count() int {
	m.mu.RLock()
	n := len(m.messages)
	m.mu.RUnlock()
	return n
}

// AllMessages returns a copy of all messages to avoid external mutation of internal state.
func (m *ArrayMemory) AllMessages() []ai.Message {
	m.mu.RLock()
	if len(m.messages) == 0 {
		m.mu.RUnlock()
		return []ai.Message{}
	}
	out := make([]ai.Message, len(m.messages))
	copy(out, m.messages)
	m.mu.RUnlock()
	return out
}

// LastMessages returns up to the last n messages as a new slice. If n <= 0, returns empty.
func (m *ArrayMemory) LastMessages(n int) []ai.Message {
	if n <= 0 {
		return []ai.Message{}
	}
	m.mu.RLock()
	if len(m.messages) == 0 {
		m.mu.RUnlock()
		return []ai.Message{}
	}
	if n > len(m.messages) {
		n = len(m.messages)
	}
	start := len(m.messages) - n
	out := make([]ai.Message, n)
	copy(out, m.messages[start:])
	m.mu.RUnlock()
	return out
}

// PopLastMessage removes and returns the last message, or nil if empty.
func (m *ArrayMemory) PopLastMessage() *ai.Message {
	m.mu.Lock()
	if len(m.messages) == 0 {
		m.mu.Unlock()
		return nil
	}
	idx := len(m.messages) - 1
	msg := m.messages[idx]
	m.messages = m.messages[:idx]
	m.mu.Unlock()
	return &msg
}

// ClearMessages removes all messages while retaining underlying capacity for efficiency.
func (m *ArrayMemory) ClearMessages() {
	m.mu.Lock()
	m.messages = m.messages[:0]
	m.mu.Unlock()
}

// FilterByRole returns a copy of messages matching the given role.
func (m *ArrayMemory) FilterByRole(role ai.MessageRole) []ai.Message {
	m.mu.RLock()
	if len(m.messages) == 0 {
		m.mu.RUnlock()
		return []ai.Message{}
	}
	filtered := make([]ai.Message, 0, len(m.messages))
	for _, msg := range m.messages {
		if msg.Role == role {
			filtered = append(filtered, msg)
		}
	}
	m.mu.RUnlock()
	if len(filtered) == 0 {
		return []ai.Message{}
	}
	out := make([]ai.Message, len(filtered))
	copy(out, filtered)
	return out
}
