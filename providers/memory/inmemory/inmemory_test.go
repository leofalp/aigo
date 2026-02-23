package inmemory

import (
	"context"
	"testing"

	"github.com/leofalp/aigo/providers/ai"
)

func TestArrayMemory_AppendAndAllMessages(t *testing.T) {
	ctx := context.Background()
	m := New()

	count, err := m.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty memory")
	}

	m.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "hi"})
	m.AppendMessage(ctx, &ai.Message{Role: ai.RoleAssistant, Content: "hello"})

	count, err = m.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 messages, got %d", count)
	}

	all, err := m.AllMessages(ctx)
	if err != nil {
		t.Fatalf("AllMessages returned unexpected error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected AllMessages to return 2, got %d", len(all))
	}

	// mutate returned slice should not affect internal state
	all[0].Content = "changed"
	allAgain, err := m.AllMessages(ctx)
	if err != nil {
		t.Fatalf("AllMessages returned unexpected error: %v", err)
	}
	if allAgain[0].Content == "changed" {
		t.Fatalf("expected copy protection in AllMessages")
	}
}

func TestArrayMemory_LastMessages(t *testing.T) {
	ctx := context.Background()
	m := New()
	for i := 0; i < 5; i++ {
		m.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: string(rune('a' + i))})
	}

	last, err := m.LastMessages(ctx, 2)
	if err != nil {
		t.Fatalf("LastMessages returned unexpected error: %v", err)
	}
	if len(last) != 2 {
		t.Fatalf("expected 2, got %d", len(last))
	}
	if last[0].Content != "d" || last[1].Content != "e" {
		t.Fatalf("unexpected last messages order: %v", last)
	}

	none, err := m.LastMessages(ctx, 0)
	if err != nil {
		t.Fatalf("LastMessages returned unexpected error: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected empty when n <= 0")
	}

	all, err := m.LastMessages(ctx, 10)
	if err != nil {
		t.Fatalf("LastMessages returned unexpected error: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("expected full slice when n > len, got %d", len(all))
	}
}

func TestArrayMemory_PopLastAndClear(t *testing.T) {
	ctx := context.Background()
	m := New()

	got, err := m.PopLastMessage(ctx)
	if err != nil {
		t.Fatalf("PopLastMessage returned unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil pop on empty")
	}

	m.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "1"})
	m.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "2"})

	last, err := m.PopLastMessage(ctx)
	if err != nil {
		t.Fatalf("PopLastMessage returned unexpected error: %v", err)
	}
	if last == nil || last.Content != "2" {
		t.Fatalf("expected to pop '2', got %#v", last)
	}

	count, err := m.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 message left, got %d", count)
	}

	m.ClearMessages(ctx)

	count, err = m.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 after clear, got %d", count)
	}
}

func TestArrayMemory_FilterByRole(t *testing.T) {
	ctx := context.Background()
	m := New()
	m.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "u1"})
	m.AppendMessage(ctx, &ai.Message{Role: ai.RoleAssistant, Content: "a1"})
	m.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "u2"})

	users, err := m.FilterByRole(ctx, ai.RoleUser)
	if err != nil {
		t.Fatalf("FilterByRole returned unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 user messages, got %d", len(users))
	}
	if users[0].Content != "u1" || users[1].Content != "u2" {
		t.Fatalf("unexpected users slice: %#v", users)
	}

	tools, err := m.FilterByRole(ctx, ai.RoleTool)
	if err != nil {
		t.Fatalf("FilterByRole returned unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Fatalf("expected 0 tool messages")
	}
}

func TestArrayMemory_AppendNilDoesNothing(t *testing.T) {
	ctx := context.Background()
	m := New()

	// append nil on empty
	m.AppendMessage(ctx, nil)
	count, err := m.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0 after appending nil on empty, got %d", count)
	}

	// append valid, then nil, ensure count not incremented by nil
	m.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "hello"})
	count, err = m.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1 after valid append, got %d", count)
	}
	m.AppendMessage(ctx, nil)
	count, err = m.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count to remain 1 after appending nil, got %d", count)
	}
}
