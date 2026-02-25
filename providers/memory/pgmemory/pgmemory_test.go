package pgmemory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// onlyQuerier is a hand-written stub that implements Querier but NOT TxQuerier
// (no Begin method). It is used to exercise the popLastMessageFallback path,
// which is taken when the db does not support transactions.
type onlyQuerier struct {
	// queryRowResult is the row returned by QueryRow.
	queryRowResult pgx.Row
	// execErr is the error returned by Exec (the DELETE step).
	execErr error
}

func (q *onlyQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, q.execErr
}

func (q *onlyQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (q *onlyQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return q.queryRowResult
}

// stubRow is a hand-written pgx.Row that returns predefined Scan results,
// used with onlyQuerier to drive popLastMessageFallback without pgxmock.
type stubRow struct {
	values []any
	err    error
}

func (r *stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, destination := range dest {
		if i >= len(r.values) {
			break
		}
		// Use reflection-free assignment via type switches for the types used
		// in popLastMessageFallback: *string (rowID/role/content), *[]byte, **string.
		switch dst := destination.(type) {
		case *string:
			if s, ok := r.values[i].(string); ok {
				*dst = s
			}
		case *[]byte:
			if b, ok := r.values[i].([]byte); ok {
				*dst = b
			}
		case **string:
			// The value can be nil (SQL NULL) or a *string pointing to a value.
			if r.values[i] == nil {
				*dst = nil
			} else if sv, ok := r.values[i].(*string); ok {
				*dst = sv
			} else if s, ok := r.values[i].(string); ok {
				*dst = &s
			}
		}
	}
	return nil
}

// TestPopLastMessage_Fallback_Success verifies that popLastMessageFallback
// correctly reads and deletes the most recent message when using a Querier
// that does not support transactions.
func TestPopLastMessage_Fallback_Success(t *testing.T) {
	row := &stubRow{
		values: []any{
			"row-id-1",  // id
			"user",      // role
			"hello",     // content
			[]byte(nil), // content_parts
			[]byte(nil), // tool_calls
			nil,         // tool_call_id (SQL NULL)
			nil,         // name (SQL NULL)
			nil,         // refusal (SQL NULL)
			nil,         // reasoning (SQL NULL)
			[]byte(nil), // code_executions
		},
	}
	querier := &onlyQuerier{queryRowResult: row}
	mem := New(querier, "session-1")

	msg, popErr := mem.PopLastMessage(context.Background())
	if popErr != nil {
		t.Fatalf("PopLastMessage fallback returned unexpected error: %v", popErr)
	}
	if msg == nil {
		t.Fatal("expected non-nil message from fallback pop")
	}
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got %q", msg.Role)
	}
	if msg.Content != "hello" {
		t.Errorf("expected content 'hello', got %q", msg.Content)
	}
}

// TestPopLastMessage_Fallback_EmptySession verifies that popLastMessageFallback
// returns (nil, nil) when the session has no messages (pgx.ErrNoRows).
func TestPopLastMessage_Fallback_EmptySession(t *testing.T) {
	row := &stubRow{err: pgx.ErrNoRows}
	querier := &onlyQuerier{queryRowResult: row}
	mem := New(querier, "session-1")

	msg, popErr := mem.PopLastMessage(context.Background())
	if popErr != nil {
		t.Fatalf("expected nil error for empty session, got: %v", popErr)
	}
	if msg != nil {
		t.Fatalf("expected nil message for empty session, got %+v", msg)
	}
}

// TestPopLastMessage_Fallback_SelectError verifies that a SELECT failure in
// popLastMessageFallback is propagated as a wrapped error.
func TestPopLastMessage_Fallback_SelectError(t *testing.T) {
	selectErr := fmt.Errorf("connection lost")
	row := &stubRow{err: selectErr}
	querier := &onlyQuerier{queryRowResult: row}
	mem := New(querier, "session-1")

	_, popErr := mem.PopLastMessage(context.Background())
	if popErr == nil {
		t.Fatal("expected error from fallback pop SELECT, got nil")
	}
	if !errors.Is(popErr, selectErr) {
		t.Errorf("expected wrapped selectErr, got %v", popErr)
	}
}

// TestPopLastMessage_Fallback_DeleteError verifies that a DELETE failure in
// popLastMessageFallback is propagated as a wrapped error.
func TestPopLastMessage_Fallback_DeleteError(t *testing.T) {
	deleteErr := fmt.Errorf("delete failed")
	row := &stubRow{
		values: []any{
			"row-id-1",
			"user",
			"hello",
			[]byte(nil),
			[]byte(nil),
			nil,
			nil,
			nil,
			nil,
			[]byte(nil),
		},
	}
	querier := &onlyQuerier{queryRowResult: row, execErr: deleteErr}
	mem := New(querier, "session-1")

	_, popErr := mem.PopLastMessage(context.Background())
	if popErr == nil {
		t.Fatal("expected error from fallback pop DELETE, got nil")
	}
	if !errors.Is(popErr, deleteErr) {
		t.Errorf("expected wrapped deleteErr, got %v", popErr)
	}
}

// TestEnsureSchema_PropagatesSeqIndexError verifies that a failure on the
// seq index creation is returned without attempting the role index.
func TestEnsureSchema_PropagatesSeqIndexError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS aigo_messages").
		WillReturnResult(pgxmock.NewResult("CREATE TABLE", 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS").
		WillReturnError(fmt.Errorf("index creation failed"))

	schemaErr := mem.EnsureSchema(context.Background())
	if schemaErr == nil {
		t.Fatal("expected error from EnsureSchema on seq index, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestEnsureSchema_PropagatesRoleIndexError verifies that a failure on the
// role index creation is returned after the table and seq index succeed.
func TestEnsureSchema_PropagatesRoleIndexError(t *testing.T) {
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
		WillReturnError(fmt.Errorf("role index creation failed"))

	schemaErr := mem.EnsureSchema(context.Background())
	if schemaErr == nil {
		t.Fatal("expected error from EnsureSchema on role index, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestScanMessages_RowsIterationError verifies that an error surfaced by
// rows.Err() after iteration is propagated by AllMessages.
func TestScanMessages_RowsIterationError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	iterErr := fmt.Errorf("network interrupted during iteration")
	columns := []string{"role", "content", "content_parts", "tool_calls", "tool_call_id", "name", "refusal", "reasoning", "code_executions"}

	// Add one valid row, then inject a close error so rows.Err() fires after the loop.
	mock.ExpectQuery("SELECT role, content").
		WithArgs("session-1").
		WillReturnRows(
			pgxmock.NewRows(columns).
				AddRow("user", "hi", nil, nil, nil, nil, nil, nil, nil).
				CloseError(iterErr),
		)

	_, queryErr := mem.AllMessages(context.Background())
	if queryErr == nil {
		t.Fatal("expected error from rows.Err(), got nil")
	}
	if !errors.Is(queryErr, iterErr) {
		t.Errorf("expected wrapped iterErr, got %v", queryErr)
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

// TestAppendMessage_ExecError verifies that an Exec error during INSERT is
// handled gracefully (logged, but no panic since AppendMessage returns no error).
func TestAppendMessage_ExecError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	mock.ExpectExec("INSERT INTO aigo_messages").
		WithArgs(
			"session-1",
			"user",
			"hello",
			[]byte(nil),
			[]byte(nil),
			"",
			"",
			"",
			"",
			[]byte(nil),
		).
		WillReturnError(fmt.Errorf("insert failed"))

	mem.AppendMessage(context.Background(), &ai.Message{
		Role:    ai.RoleUser,
		Content: "hello",
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestPopLastMessageAtomic_BeginError verifies that a Begin transaction error
// is propagated.
func TestPopLastMessageAtomic_BeginError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	mock.ExpectBegin().WillReturnError(fmt.Errorf("begin failed"))

	_, popErr := mem.PopLastMessage(context.Background())
	if popErr == nil {
		t.Fatal("expected error from Begin, got nil")
	}
	if !strings.Contains(popErr.Error(), "begin failed") {
		t.Errorf("expected 'begin failed' error, got %v", popErr)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestPopLastMessageAtomic_CommitError verifies that a Commit transaction error
// is propagated.
func TestPopLastMessageAtomic_CommitError(t *testing.T) {
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
	mock.ExpectCommit().WillReturnError(fmt.Errorf("commit failed"))

	_, popErr := mem.PopLastMessage(context.Background())
	if popErr == nil {
		t.Fatal("expected error from Commit, got nil")
	}
	if !strings.Contains(popErr.Error(), "commit failed") {
		t.Errorf("expected 'commit failed' error, got %v", popErr)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestClearMessages_ExecError verifies that an Exec error during DELETE is
// handled gracefully (logged, but no panic since ClearMessages returns no error).
func TestClearMessages_ExecError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	mem := New(mock, "session-1")

	mock.ExpectExec("DELETE FROM aigo_messages").
		WithArgs("session-1").
		WillReturnError(fmt.Errorf("delete failed"))

	mem.ClearMessages(context.Background())

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

// TestBuildMessage_MalformedToolCallsJSON verifies that malformed JSONB
// is handled gracefully by buildMessage (unmarshal error ignored, fields remain empty).
func TestBuildMessage_MalformedToolCallsJSON(t *testing.T) {
	msg := buildMessage(
		"assistant",
		"hello",
		[]byte(`{bad json}`), // content_parts
		[]byte(`{bad json}`), // tool_calls
		nil,
		nil,
		nil,
		nil,
		[]byte(`{bad json}`), // code_executions
	)

	if len(msg.ToolCalls) != 0 {
		t.Errorf("expected empty ToolCalls for malformed JSON, got %d", len(msg.ToolCalls))
	}
	if len(msg.ContentParts) != 0 {
		t.Errorf("expected empty ContentParts for malformed JSON, got %d", len(msg.ContentParts))
	}
	if len(msg.CodeExecutions) != 0 {
		t.Errorf("expected empty CodeExecutions for malformed JSON, got %d", len(msg.CodeExecutions))
	}
}
