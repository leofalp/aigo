//go:build integration

package pgmemory

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/leofalp/aigo/providers/ai"
)

// testPool is a shared connection pool created once in TestMain
// and reused across all integration test functions.
var testPool *pgxpool.Pool

// TestMain spins up a PostgreSQL container via testcontainers-go, creates the
// schema, and tears everything down after all tests complete.
func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("aigo_test"),
		postgres.WithUsername("aigo"),
		postgres.WithPassword("aigo"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("pgmemory: failed to start postgres container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("pgmemory: failed to get connection string: %v", err)
	}

	testPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("pgmemory: failed to create pool: %v", err)
	}

	// Create the schema once for all tests.
	schemaMem := New(testPool, "setup")
	if err := schemaMem.EnsureSchema(ctx); err != nil {
		log.Fatalf("pgmemory: failed to create schema: %v", err)
	}

	code := m.Run()

	testPool.Close()
	if err := testcontainers.TerminateContainer(pgContainer); err != nil {
		log.Printf("pgmemory: failed to terminate container: %v", err)
	}

	os.Exit(code)
}

// newTestMemory returns a PgMemory scoped to a unique session, guaranteeing
// test isolation without needing per-test table cleanup.
func newTestMemory(t *testing.T) *PgMemory {
	t.Helper()
	return New(testPool, "test-"+t.Name())
}

// TestPgMemory_AppendAndAllMessages verifies basic append + read-all
// round-trip, including chronological ordering and copy protection.
func TestPgMemory_AppendAndAllMessages(t *testing.T) {
	ctx := context.Background()
	mem := newTestMemory(t)

	count, err := mem.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty memory, got %d", count)
	}

	mem.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "hi"})
	mem.AppendMessage(ctx, &ai.Message{Role: ai.RoleAssistant, Content: "hello"})

	count, err = mem.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 messages, got %d", count)
	}

	allMessages, err := mem.AllMessages(ctx)
	if err != nil {
		t.Fatalf("AllMessages returned unexpected error: %v", err)
	}
	if len(allMessages) != 2 {
		t.Fatalf("expected AllMessages to return 2, got %d", len(allMessages))
	}
	if allMessages[0].Content != "hi" || allMessages[1].Content != "hello" {
		t.Fatalf("unexpected message order: %v", allMessages)
	}
	if allMessages[0].Role != ai.RoleUser || allMessages[1].Role != ai.RoleAssistant {
		t.Fatalf("unexpected roles: %v, %v", allMessages[0].Role, allMessages[1].Role)
	}
}

// TestPgMemory_LastMessages verifies the efficient last-N retrieval using
// the subquery pattern, including edge cases.
func TestPgMemory_LastMessages(t *testing.T) {
	ctx := context.Background()
	mem := newTestMemory(t)

	for i := 0; i < 5; i++ {
		mem.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: string(rune('a' + i))})
	}

	last, err := mem.LastMessages(ctx, 2)
	if err != nil {
		t.Fatalf("LastMessages returned unexpected error: %v", err)
	}
	if len(last) != 2 {
		t.Fatalf("expected 2, got %d", len(last))
	}
	if last[0].Content != "d" || last[1].Content != "e" {
		t.Fatalf("unexpected last messages order: %v", last)
	}

	none, err := mem.LastMessages(ctx, 0)
	if err != nil {
		t.Fatalf("LastMessages returned unexpected error: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected empty when n <= 0, got %d", len(none))
	}

	allMessages, err := mem.LastMessages(ctx, 10)
	if err != nil {
		t.Fatalf("LastMessages returned unexpected error: %v", err)
	}
	if len(allMessages) != 5 {
		t.Fatalf("expected full slice when n > len, got %d", len(allMessages))
	}
}

// TestPgMemory_PopLastAndClear verifies atomic pop and session-wide clear.
func TestPgMemory_PopLastAndClear(t *testing.T) {
	ctx := context.Background()
	mem := newTestMemory(t)

	// Pop on empty returns nil.
	got, err := mem.PopLastMessage(ctx)
	if err != nil {
		t.Fatalf("PopLastMessage returned unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil pop on empty, got %v", got)
	}

	mem.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "1"})
	mem.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "2"})

	last, err := mem.PopLastMessage(ctx)
	if err != nil {
		t.Fatalf("PopLastMessage returned unexpected error: %v", err)
	}
	if last == nil || last.Content != "2" {
		t.Fatalf("expected to pop '2', got %#v", last)
	}

	count, err := mem.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 message left, got %d", count)
	}

	mem.ClearMessages(ctx)

	count, err = mem.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 after clear, got %d", count)
	}
}

// TestPgMemory_FilterByRole verifies role-based message retrieval.
func TestPgMemory_FilterByRole(t *testing.T) {
	ctx := context.Background()
	mem := newTestMemory(t)

	mem.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "u1"})
	mem.AppendMessage(ctx, &ai.Message{Role: ai.RoleAssistant, Content: "a1"})
	mem.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "u2"})

	users, err := mem.FilterByRole(ctx, ai.RoleUser)
	if err != nil {
		t.Fatalf("FilterByRole returned unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 user messages, got %d", len(users))
	}
	if users[0].Content != "u1" || users[1].Content != "u2" {
		t.Fatalf("unexpected users slice: %#v", users)
	}

	tools, err := mem.FilterByRole(ctx, ai.RoleTool)
	if err != nil {
		t.Fatalf("FilterByRole returned unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Fatalf("expected 0 tool messages, got %d", len(tools))
	}
}

// TestPgMemory_AppendNilDoesNothing verifies nil messages are silently ignored.
func TestPgMemory_AppendNilDoesNothing(t *testing.T) {
	ctx := context.Background()
	mem := newTestMemory(t)

	// Append nil on empty.
	mem.AppendMessage(ctx, nil)
	count, err := mem.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0 after appending nil on empty, got %d", count)
	}

	// Append valid, then nil.
	mem.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "hello"})
	mem.AppendMessage(ctx, nil)
	count, err = mem.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count to remain 1 after appending nil, got %d", count)
	}
}

// TestPgMemory_SessionIsolation verifies that messages from different sessions
// do not leak into each other's results.
func TestPgMemory_SessionIsolation(t *testing.T) {
	ctx := context.Background()
	sessionA := New(testPool, "isolation-session-a-"+t.Name())
	sessionB := New(testPool, "isolation-session-b-"+t.Name())

	sessionA.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "from A"})
	sessionB.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "from B"})

	messagesA, err := sessionA.AllMessages(ctx)
	if err != nil {
		t.Fatalf("AllMessages for session A returned error: %v", err)
	}
	if len(messagesA) != 1 || messagesA[0].Content != "from A" {
		t.Fatalf("session A should only see its own message, got: %v", messagesA)
	}

	messagesB, err := sessionB.AllMessages(ctx)
	if err != nil {
		t.Fatalf("AllMessages for session B returned error: %v", err)
	}
	if len(messagesB) != 1 || messagesB[0].Content != "from B" {
		t.Fatalf("session B should only see its own message, got: %v", messagesB)
	}

	countA, err := sessionA.Count(ctx)
	if err != nil {
		t.Fatalf("Count for session A returned error: %v", err)
	}
	if countA != 1 {
		t.Fatalf("expected session A count 1, got %d", countA)
	}
}

// TestPgMemory_ToolCallRoundTrip verifies that messages with tool calls
// survive the JSON serialization round-trip through JSONB columns.
func TestPgMemory_ToolCallRoundTrip(t *testing.T) {
	ctx := context.Background()
	mem := newTestMemory(t)

	// Store an assistant message with tool calls.
	toolCallMsg := &ai.Message{
		Role:    ai.RoleAssistant,
		Content: "",
		ToolCalls: []ai.ToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: ai.ToolCallFunction{
					Name:      "get_weather",
					Arguments: `{"location": "San Francisco"}`,
				},
			},
		},
	}
	mem.AppendMessage(ctx, toolCallMsg)

	// Store the tool response.
	toolResponseMsg := &ai.Message{
		Role:       ai.RoleTool,
		Content:    `{"temperature": 72}`,
		ToolCallID: "call_123",
		Name:       "get_weather",
	}
	mem.AppendMessage(ctx, toolResponseMsg)

	allMessages, err := mem.AllMessages(ctx)
	if err != nil {
		t.Fatalf("AllMessages returned unexpected error: %v", err)
	}
	if len(allMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(allMessages))
	}

	// Verify tool call round-trip.
	retrieved := allMessages[0]
	if len(retrieved.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(retrieved.ToolCalls))
	}
	if retrieved.ToolCalls[0].ID != "call_123" {
		t.Fatalf("expected tool call ID 'call_123', got '%s'", retrieved.ToolCalls[0].ID)
	}
	if retrieved.ToolCalls[0].Function.Name != "get_weather" {
		t.Fatalf("expected function name 'get_weather', got '%s'", retrieved.ToolCalls[0].Function.Name)
	}
	if retrieved.ToolCalls[0].Function.Arguments != `{"location": "San Francisco"}` {
		t.Fatalf("expected function arguments preserved, got '%s'", retrieved.ToolCalls[0].Function.Arguments)
	}

	// Verify tool response round-trip.
	toolResp := allMessages[1]
	if toolResp.ToolCallID != "call_123" {
		t.Fatalf("expected tool_call_id 'call_123', got '%s'", toolResp.ToolCallID)
	}
	if toolResp.Name != "get_weather" {
		t.Fatalf("expected tool name 'get_weather', got '%s'", toolResp.Name)
	}
	if toolResp.Content != `{"temperature": 72}` {
		t.Fatalf("expected tool content preserved, got '%s'", toolResp.Content)
	}
}

// TestPgMemory_ReasoningAndRefusalRoundTrip verifies that extended message
// fields (reasoning, refusal) survive the PostgreSQL round-trip.
func TestPgMemory_ReasoningAndRefusalRoundTrip(t *testing.T) {
	ctx := context.Background()
	mem := newTestMemory(t)

	mem.AppendMessage(ctx, &ai.Message{
		Role:      ai.RoleAssistant,
		Content:   "I can help with that.",
		Reasoning: "The user is asking about weather, which is a safe topic.",
	})
	mem.AppendMessage(ctx, &ai.Message{
		Role:    ai.RoleAssistant,
		Content: "",
		Refusal: "I cannot assist with that request.",
	})

	allMessages, err := mem.AllMessages(ctx)
	if err != nil {
		t.Fatalf("AllMessages returned unexpected error: %v", err)
	}
	if len(allMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(allMessages))
	}

	if allMessages[0].Reasoning != "The user is asking about weather, which is a safe topic." {
		t.Fatalf("expected reasoning preserved, got '%s'", allMessages[0].Reasoning)
	}
	if allMessages[1].Refusal != "I cannot assist with that request." {
		t.Fatalf("expected refusal preserved, got '%s'", allMessages[1].Refusal)
	}
}

// TestPgMemory_WithTableName verifies that a custom table name is respected.
func TestPgMemory_WithTableName(t *testing.T) {
	ctx := context.Background()
	customTable := "custom_messages"

	mem := New(testPool, "custom-"+t.Name(), WithTableName(customTable))

	// Create the custom table.
	if err := mem.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema for custom table returned error: %v", err)
	}

	mem.AppendMessage(ctx, &ai.Message{Role: ai.RoleUser, Content: "custom table test"})

	count, err := mem.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 message in custom table, got %d", count)
	}

	// Clean up the custom table after the test.
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "DROP TABLE IF EXISTS "+customTable)
	})
}
