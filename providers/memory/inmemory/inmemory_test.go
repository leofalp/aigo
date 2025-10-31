package inmemory

import (
	"aigo/providers/ai"
	"testing"
)

func TestArrayMemory_AppendAndAllMessages(t *testing.T) {
	m := NewArrayMemory()
	if m.Count() != 0 {
		t.Fatalf("expected empty memory")
	}

	m.AppendMessage(&ai.Message{Role: ai.RoleUser, Content: "hi"})
	m.AppendMessage(&ai.Message{Role: ai.RoleAssistant, Content: "hello"})

	if m.Count() != 2 {
		t.Fatalf("expected 2 messages, got %d", m.Count())
	}

	all := m.AllMessages()
	if len(all) != 2 {
		t.Fatalf("expected AllMessages to return 2, got %d", len(all))
	}

	// mutate returned slice should not affect internal state
	all[0].Content = "changed"
	if m.AllMessages()[0].Content == "changed" {
		t.Fatalf("expected copy protection in AllMessages")
	}
}

func TestArrayMemory_LastMessages(t *testing.T) {
	m := NewArrayMemory()
	for i := 0; i < 5; i++ {
		m.AppendMessage(&ai.Message{Role: ai.RoleUser, Content: string(rune('a' + i))})
	}

	last := m.LastMessages(2)
	if len(last) != 2 {
		t.Fatalf("expected 2, got %d", len(last))
	}
	if last[0].Content != "d" || last[1].Content != "e" {
		t.Fatalf("unexpected last messages order: %v", last)
	}

	none := m.LastMessages(0)
	if len(none) != 0 {
		t.Fatalf("expected empty when n <= 0")
	}

	all := m.LastMessages(10)
	if len(all) != 5 {
		t.Fatalf("expected full slice when n > len, got %d", len(all))
	}
}

func TestArrayMemory_PopLastAndClear(t *testing.T) {
	m := NewArrayMemory()
	if got := m.PopLastMessage(); got != nil {
		t.Fatalf("expected nil pop on empty")
	}

	m.AppendMessage(&ai.Message{Role: ai.RoleUser, Content: "1"})
	m.AppendMessage(&ai.Message{Role: ai.RoleUser, Content: "2"})

	last := m.PopLastMessage()
	if last == nil || last.Content != "2" {
		t.Fatalf("expected to pop '2', got %#v", last)
	}
	if m.Count() != 1 {
		t.Fatalf("expected 1 message left, got %d", m.Count())
	}

	m.ClearMessages()
	if m.Count() != 0 {
		t.Fatalf("expected 0 after clear, got %d", m.Count())
	}
}

func TestArrayMemory_FilterByRole(t *testing.T) {
	m := NewArrayMemory()
	m.AppendMessage(&ai.Message{Role: ai.RoleUser, Content: "u1"})
	m.AppendMessage(&ai.Message{Role: ai.RoleAssistant, Content: "a1"})
	m.AppendMessage(&ai.Message{Role: ai.RoleUser, Content: "u2"})

	users := m.FilterByRole(ai.RoleUser)
	if len(users) != 2 {
		t.Fatalf("expected 2 user messages, got %d", len(users))
	}
	if users[0].Content != "u1" || users[1].Content != "u2" {
		t.Fatalf("unexpected users slice: %#v", users)
	}

	tools := m.FilterByRole(ai.RoleTool)
	if len(tools) != 0 {
		t.Fatalf("expected 0 tool messages")
	}
}

func TestArrayMemory_AppendNilDoesNothing(t *testing.T) {
	m := NewArrayMemory()

	// append nil on empty
	m.AppendMessage(nil)
	if m.Count() != 0 {
		t.Fatalf("expected count 0 after appending nil on empty, got %d", m.Count())
	}

	// append valid, then nil, ensure count not incremented by nil
	m.AppendMessage(&ai.Message{Role: ai.RoleUser, Content: "hello"})
	if m.Count() != 1 {
		t.Fatalf("expected count 1 after valid append, got %d", m.Count())
	}
	m.AppendMessage(nil)
	if m.Count() != 1 {
		t.Fatalf("expected count to remain 1 after appending nil, got %d", m.Count())
	}
}
