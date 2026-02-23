package pgmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/pashagolub/pgxmock/v4"

	"github.com/leofalp/aigo/providers/ai"
)

// TestNew_Defaults verifies that New applies the default table name and
// correctly stores the session ID.
func TestNew_Defaults(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")
	if mem.tableName != defaultTableName {
		t.Fatalf("expected default table name %q, got %q", defaultTableName, mem.tableName)
	}
	if mem.sessionID != "session-1" {
		t.Fatalf("expected session ID %q, got %q", "session-1", mem.sessionID)
	}
}

// TestNew_WithTableName verifies that WithTableName overrides the default
// and sanitizes the name via pgx.Identifier.
func TestNew_WithTableName(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1", WithTableName("custom_table"))

	// pgx.Identifier.Sanitize() quotes the name: "custom_table"
	expected := `"custom_table"`
	if mem.tableName != expected {
		t.Fatalf("expected table name %q, got %q", expected, mem.tableName)
	}
}

// TestAppendMessage_NilIsIgnored verifies that a nil message does not trigger
// any database interaction.
func TestAppendMessage_NilIsIgnored(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")
	mem.AppendMessage(context.Background(), nil)

	// No expectations set — pgxmock will fail if any query is executed.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected database call for nil message: %v", err)
	}
}

// TestAppendMessage_SimpleMessage verifies that a plain text message triggers
// the correct INSERT with the right parameters.
func TestAppendMessage_SimpleMessage(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	mock.ExpectExec("INSERT INTO aigo_messages").
		WithArgs(
			"session-1",   // session_id
			"user",        // role
			"hello world", // content
			[]byte(nil),   // content_parts — typed nil []byte matches marshalNullableJSON output
			[]byte(nil),   // tool_calls — typed nil []byte matches marshalNullableJSON output
			"",            // tool_call_id
			"",            // name
			"",            // refusal
			"",            // reasoning
			[]byte(nil),   // code_executions — typed nil []byte matches marshalNullableJSON output
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mem.AppendMessage(context.Background(), &ai.Message{
		Role:    ai.RoleUser,
		Content: "hello world",
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestAppendMessage_WithToolCalls verifies JSONB serialization of tool calls.
func TestAppendMessage_WithToolCalls(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	toolCalls := []ai.ToolCall{{
		ID:   "call_1",
		Type: "function",
		Function: ai.ToolCallFunction{
			Name:      "get_weather",
			Arguments: `{"city":"NYC"}`,
		},
	}}
	toolCallsJSON, _ := json.Marshal(toolCalls)

	mock.ExpectExec("INSERT INTO aigo_messages").
		WithArgs(
			"session-1",
			"assistant",
			"",
			[]byte(nil),   // content_parts — typed nil []byte matches marshalNullableJSON output
			toolCallsJSON, // tool_calls serialized
			"",
			"",
			"",
			"",
			[]byte(nil), // code_executions — typed nil []byte matches marshalNullableJSON output
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mem.AppendMessage(context.Background(), &ai.Message{
		Role:      ai.RoleAssistant,
		ToolCalls: toolCalls,
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestCount_ReturnsCorrectValue verifies Count scans the row correctly.
func TestCount_ReturnsCorrectValue(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	mock.ExpectQuery("SELECT COUNT").
		WithArgs("session-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(42))

	count, queryErr := mem.Count(context.Background())
	if queryErr != nil {
		t.Fatalf("Count returned unexpected error: %v", queryErr)
	}
	if count != 42 {
		t.Fatalf("expected count 42, got %d", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestCount_PropagatesError verifies that database errors are wrapped and returned.
func TestCount_PropagatesError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	mock.ExpectQuery("SELECT COUNT").
		WithArgs("session-1").
		WillReturnError(fmt.Errorf("connection refused"))

	_, queryErr := mem.Count(context.Background())
	if queryErr == nil {
		t.Fatalf("expected error from Count, got nil")
	}
}

// TestAllMessages_ReturnsChronologicalOrder verifies that rows are scanned
// into ai.Message values in the correct order.
func TestAllMessages_ReturnsChronologicalOrder(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	columns := []string{"role", "content", "content_parts", "tool_calls", "tool_call_id", "name", "refusal", "reasoning", "code_executions"}
	mock.ExpectQuery("SELECT role, content").
		WithArgs("session-1").
		WillReturnRows(
			pgxmock.NewRows(columns).
				AddRow("user", "hi", nil, nil, nil, nil, nil, nil, nil).
				AddRow("assistant", "hello", nil, nil, nil, nil, nil, nil, nil),
		)

	messages, queryErr := mem.AllMessages(context.Background())
	if queryErr != nil {
		t.Fatalf("AllMessages returned unexpected error: %v", queryErr)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != ai.RoleUser || messages[0].Content != "hi" {
		t.Fatalf("unexpected first message: %+v", messages[0])
	}
	if messages[1].Role != ai.RoleAssistant || messages[1].Content != "hello" {
		t.Fatalf("unexpected second message: %+v", messages[1])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestAllMessages_EmptyResult verifies that an empty result set returns a
// non-nil empty slice.
func TestAllMessages_EmptyResult(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	columns := []string{"role", "content", "content_parts", "tool_calls", "tool_call_id", "name", "refusal", "reasoning", "code_executions"}
	mock.ExpectQuery("SELECT role, content").
		WithArgs("session-1").
		WillReturnRows(pgxmock.NewRows(columns))

	messages, queryErr := mem.AllMessages(context.Background())
	if queryErr != nil {
		t.Fatalf("AllMessages returned unexpected error: %v", queryErr)
	}
	if messages == nil {
		t.Fatalf("expected non-nil slice for empty result")
	}
	if len(messages) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(messages))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestLastMessages_ZeroOrNegative verifies that n <= 0 returns empty without
// hitting the database.
func TestLastMessages_ZeroOrNegative(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	messages, queryErr := mem.LastMessages(context.Background(), 0)
	if queryErr != nil {
		t.Fatalf("LastMessages returned unexpected error: %v", queryErr)
	}
	if len(messages) != 0 {
		t.Fatalf("expected empty slice for n=0, got %d", len(messages))
	}

	messages, queryErr = mem.LastMessages(context.Background(), -1)
	if queryErr != nil {
		t.Fatalf("LastMessages returned unexpected error: %v", queryErr)
	}
	if len(messages) != 0 {
		t.Fatalf("expected empty slice for n=-1, got %d", len(messages))
	}

	// No DB expectations — pgxmock will fail if any query is executed.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected database call for n <= 0: %v", err)
	}
}

// TestLastMessages_ReturnsCorrectSubset verifies the subquery pattern returns
// the correct messages in chronological order.
func TestLastMessages_ReturnsCorrectSubset(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	columns := []string{"role", "content", "content_parts", "tool_calls", "tool_call_id", "name", "refusal", "reasoning", "code_executions"}
	mock.ExpectQuery("SELECT role, content").
		WithArgs("session-1", 2).
		WillReturnRows(
			pgxmock.NewRows(columns).
				AddRow("user", "d", nil, nil, nil, nil, nil, nil, nil).
				AddRow("user", "e", nil, nil, nil, nil, nil, nil, nil),
		)

	messages, queryErr := mem.LastMessages(context.Background(), 2)
	if queryErr != nil {
		t.Fatalf("LastMessages returned unexpected error: %v", queryErr)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Content != "d" || messages[1].Content != "e" {
		t.Fatalf("unexpected messages: %v", messages)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestPopLastMessage_Atomic verifies the transaction-based pop path when the
// db implements TxQuerier (pgxmock.NewPool satisfies this).
func TestPopLastMessage_Atomic(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	columns := []string{"role", "content", "content_parts", "tool_calls", "tool_call_id", "name", "refusal", "reasoning", "code_executions"}

	mock.ExpectBegin()
	mock.ExpectQuery("DELETE FROM aigo_messages").
		WithArgs("session-1").
		WillReturnRows(
			pgxmock.NewRows(columns).
				AddRow("user", "last message", nil, nil, nil, nil, nil, nil, nil),
		)
	mock.ExpectCommit()

	msg, popErr := mem.PopLastMessage(context.Background())
	if popErr != nil {
		t.Fatalf("PopLastMessage returned unexpected error: %v", popErr)
	}
	if msg == nil {
		t.Fatalf("expected non-nil message from pop")
	}
	if msg.Content != "last message" {
		t.Fatalf("expected content 'last message', got %q", msg.Content)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestPopLastMessage_EmptySession verifies that popping from an empty session
// returns (nil, nil) without error.
func TestPopLastMessage_EmptySession(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	columns := []string{"role", "content", "content_parts", "tool_calls", "tool_call_id", "name", "refusal", "reasoning", "code_executions"}

	mock.ExpectBegin()
	mock.ExpectQuery("DELETE FROM aigo_messages").
		WithArgs("session-1").
		WillReturnRows(pgxmock.NewRows(columns)) // no rows
	mock.ExpectRollback()

	msg, popErr := mem.PopLastMessage(context.Background())
	if popErr != nil {
		t.Fatalf("PopLastMessage returned unexpected error: %v", popErr)
	}
	if msg != nil {
		t.Fatalf("expected nil message for empty session, got %+v", msg)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestClearMessages_ExecutesDelete verifies that ClearMessages issues a DELETE
// scoped to the session.
func TestClearMessages_ExecutesDelete(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	mock.ExpectExec("DELETE FROM aigo_messages").
		WithArgs("session-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 5))

	mem.ClearMessages(context.Background())

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestFilterByRole_ReturnsMatchingMessages verifies that FilterByRole passes
// the correct role parameter and scans results.
func TestFilterByRole_ReturnsMatchingMessages(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	columns := []string{"role", "content", "content_parts", "tool_calls", "tool_call_id", "name", "refusal", "reasoning", "code_executions"}
	mock.ExpectQuery("SELECT role, content").
		WithArgs("session-1", "user").
		WillReturnRows(
			pgxmock.NewRows(columns).
				AddRow("user", "u1", nil, nil, nil, nil, nil, nil, nil).
				AddRow("user", "u2", nil, nil, nil, nil, nil, nil, nil),
		)

	messages, queryErr := mem.FilterByRole(context.Background(), ai.RoleUser)
	if queryErr != nil {
		t.Fatalf("FilterByRole returned unexpected error: %v", queryErr)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Content != "u1" || messages[1].Content != "u2" {
		t.Fatalf("unexpected messages: %v", messages)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestFilterByRole_NoMatches verifies that an empty result set returns a
// non-nil empty slice.
func TestFilterByRole_NoMatches(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	columns := []string{"role", "content", "content_parts", "tool_calls", "tool_call_id", "name", "refusal", "reasoning", "code_executions"}
	mock.ExpectQuery("SELECT role, content").
		WithArgs("session-1", "tool").
		WillReturnRows(pgxmock.NewRows(columns))

	messages, queryErr := mem.FilterByRole(context.Background(), ai.RoleTool)
	if queryErr != nil {
		t.Fatalf("FilterByRole returned unexpected error: %v", queryErr)
	}
	if messages == nil {
		t.Fatalf("expected non-nil slice for no matches")
	}
	if len(messages) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(messages))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestToolCallRoundTrip_JSONDeserialization verifies that tool call JSONB
// stored in rows is correctly deserialized back into ai.ToolCall structs.
func TestToolCallRoundTrip_JSONDeserialization(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	toolCalls := []ai.ToolCall{{
		ID:   "call_abc",
		Type: "function",
		Function: ai.ToolCallFunction{
			Name:      "search",
			Arguments: `{"query":"test"}`,
		},
	}}
	toolCallsJSON, _ := json.Marshal(toolCalls)

	toolCallID := "call_abc"
	toolName := "search"

	columns := []string{"role", "content", "content_parts", "tool_calls", "tool_call_id", "name", "refusal", "reasoning", "code_executions"}
	mock.ExpectQuery("SELECT role, content").
		WithArgs("session-1").
		WillReturnRows(
			pgxmock.NewRows(columns).
				AddRow("assistant", "", nil, toolCallsJSON, nil, nil, nil, nil, nil).
				AddRow("tool", `{"result":"found"}`, nil, nil, &toolCallID, &toolName, nil, nil, nil),
		)

	messages, queryErr := mem.AllMessages(context.Background())
	if queryErr != nil {
		t.Fatalf("AllMessages returned unexpected error: %v", queryErr)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// Verify assistant message tool calls.
	if len(messages[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(messages[0].ToolCalls))
	}
	if messages[0].ToolCalls[0].ID != "call_abc" {
		t.Fatalf("expected tool call ID 'call_abc', got %q", messages[0].ToolCalls[0].ID)
	}
	if messages[0].ToolCalls[0].Function.Name != "search" {
		t.Fatalf("expected function name 'search', got %q", messages[0].ToolCalls[0].Function.Name)
	}

	// Verify tool response message.
	if messages[1].ToolCallID != "call_abc" {
		t.Fatalf("expected tool_call_id 'call_abc', got %q", messages[1].ToolCallID)
	}
	if messages[1].Name != "search" {
		t.Fatalf("expected name 'search', got %q", messages[1].Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestEnsureSchema_ExecutesAllStatements verifies that EnsureSchema issues
// the CREATE TABLE and CREATE INDEX statements.
func TestEnsureSchema_ExecutesAllStatements(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS aigo_messages").
		WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS").
		WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS").
		WillReturnResult(pgxmock.NewResult("CREATE INDEX", 0))

	if schemaErr := mem.EnsureSchema(context.Background()); schemaErr != nil {
		t.Fatalf("EnsureSchema returned unexpected error: %v", schemaErr)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestEnsureSchema_PropagatesTableError verifies that a table creation failure
// is returned without attempting index creation.
func TestEnsureSchema_PropagatesTableError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS aigo_messages").
		WillReturnError(fmt.Errorf("permission denied"))

	schemaErr := mem.EnsureSchema(context.Background())
	if schemaErr == nil {
		t.Fatalf("expected error from EnsureSchema, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestMarshalNullableJSON_EmptySlicesReturnNil verifies that empty slices
// produce nil (SQL NULL) instead of "[]".
func TestMarshalNullableJSON_EmptySlicesReturnNil(t *testing.T) {
	testCases := []struct {
		name  string
		value any
	}{
		{"empty ToolCall slice", []ai.ToolCall{}},
		{"nil ToolCall slice", []ai.ToolCall(nil)},
		{"empty ContentPart slice", []ai.ContentPart{}},
		{"nil ContentPart slice", []ai.ContentPart(nil)},
		{"empty CodeExecution slice", []ai.CodeExecution{}},
		{"nil CodeExecution slice", []ai.CodeExecution(nil)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := marshalNullableJSON(testCase.value)
			if err != nil {
				t.Fatalf("marshalNullableJSON returned unexpected error: %v", err)
			}
			if result != nil {
				t.Fatalf("expected nil for %s, got %s", testCase.name, string(result))
			}
		})
	}
}

// TestMarshalNullableJSON_NonEmptySlicesReturnJSON verifies that populated
// slices produce valid JSON.
func TestMarshalNullableJSON_NonEmptySlicesReturnJSON(t *testing.T) {
	toolCalls := []ai.ToolCall{{ID: "call_1", Type: "function"}}
	result, err := marshalNullableJSON(toolCalls)
	if err != nil {
		t.Fatalf("marshalNullableJSON returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil JSON for non-empty slice")
	}

	var deserialized []ai.ToolCall
	if err := json.Unmarshal(result, &deserialized); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(deserialized) != 1 || deserialized[0].ID != "call_1" {
		t.Fatalf("unexpected deserialized result: %v", deserialized)
	}
}

// TestDerefString verifies nil and non-nil pointer dereferencing.
func TestDerefString(t *testing.T) {
	if result := derefString(nil); result != "" {
		t.Fatalf("expected empty string for nil, got %q", result)
	}

	value := "hello"
	if result := derefString(&value); result != "hello" {
		t.Fatalf("expected %q, got %q", "hello", result)
	}
}
