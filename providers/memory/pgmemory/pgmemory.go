package pgmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/memory"
)

// defaultTableName is the PostgreSQL table used when no custom name is provided.
const defaultTableName = "aigo_messages"

// Querier abstracts the pgx query methods needed by PgMemory.
// Both *pgxpool.Pool and pgx.Tx satisfy this interface, allowing
// callers to inject either a connection pool or a single transaction.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxQuerier extends Querier with transaction support. *pgxpool.Pool satisfies
// this interface but pgx.Tx does not. Methods that require atomicity (e.g.,
// PopLastMessage) attempt a type assertion to TxQuerier and fall back to a
// non-atomic path when only Querier is available.
type TxQuerier interface {
	Querier
	Begin(ctx context.Context) (pgx.Tx, error)
}

// PgMemory implements [memory.Provider] with PostgreSQL persistence.
// Each instance is scoped to a single session (conversation or thread).
// Thread safety is handled by the underlying pgx connection pool; no
// application-level mutex is needed.
type PgMemory struct {
	db        Querier
	sessionID string
	tableName string
}

// Compile-time check: PgMemory must implement memory.Provider.
var _ memory.Provider = (*PgMemory)(nil)

// Option configures optional PgMemory behavior.
type Option func(*PgMemory)

// WithTableName overrides the default table name ("aigo_messages").
// The name is sanitized via pgx.Identifier to prevent SQL injection,
// since it is interpolated into queries via fmt.Sprintf.
func WithTableName(name string) Option {
	return func(m *PgMemory) {
		m.tableName = pgx.Identifier{name}.Sanitize()
	}
}

// New creates a PostgreSQL-backed memory provider for the given session.
// The db parameter must be a pgx-compatible query executor (typically
// *pgxpool.Pool). The sessionID scopes all reads and writes to a single
// conversation thread.
func New(db Querier, sessionID string, opts ...Option) *PgMemory {
	pgMemory := &PgMemory{
		db:        db,
		sessionID: sessionID,
		tableName: defaultTableName,
	}
	for _, opt := range opts {
		opt(pgMemory)
	}
	return pgMemory
}

// AppendMessage persists a message to PostgreSQL. A nil message is silently
// ignored to match the memory.Provider contract. JSONB fields (tool_calls,
// content_parts, code_executions) are serialized with encoding/json.
func (m *PgMemory) AppendMessage(ctx context.Context, message *ai.Message) {
	if message == nil {
		return
	}

	toolCallsJSON, _ := marshalNullableJSON(message.ToolCalls)
	contentPartsJSON, _ := marshalNullableJSON(message.ContentParts)
	codeExecutionsJSON, _ := marshalNullableJSON(message.CodeExecutions)

	query := fmt.Sprintf(`INSERT INTO %s
		(session_id, role, content, content_parts, tool_calls, tool_call_id, name, refusal, reasoning, code_executions)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, m.tableName)

	_, err := m.db.Exec(ctx, query,
		m.sessionID,
		string(message.Role),
		message.Content,
		contentPartsJSON,
		toolCallsJSON,
		message.ToolCallID,
		message.Name,
		message.Refusal,
		message.Reasoning,
		codeExecutionsJSON,
	)
	if err != nil {
		// AppendMessage has no error return per the memory.Provider interface.
		// Log the error so it isn't swallowed silently.
		slog.Error("pgmemory: failed to append message", "session_id", m.sessionID, "error", err)
	}
}

// Count returns the number of messages stored for this session.
func (m *PgMemory) Count(ctx context.Context) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE session_id = $1`, m.tableName)

	var count int
	if err := m.db.QueryRow(ctx, query, m.sessionID).Scan(&count); err != nil {
		return 0, fmt.Errorf("pgmemory: count: %w", err)
	}
	return count, nil
}

// AllMessages returns all messages for this session in chronological order
// (ordered by the monotonic seq column).
func (m *PgMemory) AllMessages(ctx context.Context) ([]ai.Message, error) {
	query := fmt.Sprintf(`SELECT role, content, content_parts, tool_calls, tool_call_id, name, refusal, reasoning, code_executions
		FROM %s WHERE session_id = $1 ORDER BY seq ASC`, m.tableName)

	rows, err := m.db.Query(ctx, query, m.sessionID)
	if err != nil {
		return nil, fmt.Errorf("pgmemory: all messages: %w", err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

// LastMessages returns the last n messages in chronological order using an
// efficient SQL pattern: fetch the n most recent rows (ORDER BY seq DESC
// LIMIT n), then reverse them so the caller receives oldest-first order.
// Returns an empty slice when n is zero or negative.
func (m *PgMemory) LastMessages(ctx context.Context, n int) ([]ai.Message, error) {
	if n <= 0 {
		return []ai.Message{}, nil
	}

	// Subquery fetches newest-first, outer query re-orders oldest-first.
	query := fmt.Sprintf(`SELECT role, content, content_parts, tool_calls, tool_call_id, name, refusal, reasoning, code_executions
		FROM (
			SELECT seq, role, content, content_parts, tool_calls, tool_call_id, name, refusal, reasoning, code_executions
			FROM %s WHERE session_id = $1 ORDER BY seq DESC LIMIT $2
		) sub ORDER BY sub.seq ASC`, m.tableName)

	rows, err := m.db.Query(ctx, query, m.sessionID, n)
	if err != nil {
		return nil, fmt.Errorf("pgmemory: last messages: %w", err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

// PopLastMessage removes and returns the most recent message for this session.
// When the underlying db implements TxQuerier (e.g., *pgxpool.Pool), the
// operation is atomic (BEGIN → DELETE … RETURNING → COMMIT). Otherwise a
// non-atomic fallback (SELECT + DELETE) is used.
// Returns (nil, nil) when the session has no messages.
func (m *PgMemory) PopLastMessage(ctx context.Context) (*ai.Message, error) {
	if txDB, ok := m.db.(TxQuerier); ok {
		return m.popLastMessageAtomic(ctx, txDB)
	}
	return m.popLastMessageFallback(ctx)
}

// popLastMessageAtomic uses a transaction to atomically delete and return
// the most recent message, preventing race conditions under concurrent access.
func (m *PgMemory) popLastMessageAtomic(ctx context.Context, txDB TxQuerier) (*ai.Message, error) {
	tx, err := txDB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("pgmemory: pop begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	query := fmt.Sprintf(`DELETE FROM %s
		WHERE id = (
			SELECT id FROM %s WHERE session_id = $1 ORDER BY seq DESC LIMIT 1
		)
		RETURNING role, content, content_parts, tool_calls, tool_call_id, name, refusal, reasoning, code_executions`,
		m.tableName, m.tableName)

	msg, err := scanSingleMessage(tx.QueryRow(ctx, query, m.sessionID))
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, nil
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("pgmemory: pop commit tx: %w", err)
	}
	return msg, nil
}

// popLastMessageFallback performs a non-atomic SELECT then DELETE when the
// db does not support transactions (i.e., only implements Querier).
func (m *PgMemory) popLastMessageFallback(ctx context.Context) (*ai.Message, error) {
	// First, find the row to delete.
	selectQuery := fmt.Sprintf(`SELECT id, role, content, content_parts, tool_calls, tool_call_id, name, refusal, reasoning, code_executions
		FROM %s WHERE session_id = $1 ORDER BY seq DESC LIMIT 1`, m.tableName)

	var rowID string
	var role, content string
	var contentPartsJSON, toolCallsJSON, codeExecutionsJSON []byte
	var toolCallID, name, refusal, reasoning *string

	err := m.db.QueryRow(ctx, selectQuery, m.sessionID).Scan(
		&rowID, &role, &content, &contentPartsJSON, &toolCallsJSON,
		&toolCallID, &name, &refusal, &reasoning, &codeExecutionsJSON,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("pgmemory: pop select: %w", err)
	}

	// Delete by primary key.
	deleteQuery := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, m.tableName)
	if _, err := m.db.Exec(ctx, deleteQuery, rowID); err != nil {
		return nil, fmt.Errorf("pgmemory: pop delete: %w", err)
	}

	msg := buildMessage(role, content, contentPartsJSON, toolCallsJSON, toolCallID, name, refusal, reasoning, codeExecutionsJSON)
	return &msg, nil
}

// ClearMessages deletes all messages for this session.
func (m *PgMemory) ClearMessages(ctx context.Context) {
	query := fmt.Sprintf(`DELETE FROM %s WHERE session_id = $1`, m.tableName)
	if _, err := m.db.Exec(ctx, query, m.sessionID); err != nil {
		// ClearMessages has no error return per the memory.Provider interface.
		// Log the error so it isn't swallowed silently.
		slog.Error("pgmemory: failed to clear messages", "session_id", m.sessionID, "error", err)
	}
}

// FilterByRole returns all messages matching the given role for this session,
// in chronological order. Returns an empty slice when no messages match.
func (m *PgMemory) FilterByRole(ctx context.Context, role ai.MessageRole) ([]ai.Message, error) {
	query := fmt.Sprintf(`SELECT role, content, content_parts, tool_calls, tool_call_id, name, refusal, reasoning, code_executions
		FROM %s WHERE session_id = $1 AND role = $2 ORDER BY seq ASC`, m.tableName)

	rows, err := m.db.Query(ctx, query, m.sessionID, string(role))
	if err != nil {
		return nil, fmt.Errorf("pgmemory: filter by role: %w", err)
	}
	defer rows.Close()

	return scanMessages(rows)
}

// scanMessages iterates over pgx.Rows and returns a slice of ai.Message.
// Returns an empty non-nil slice when no rows are present.
func scanMessages(rows pgx.Rows) ([]ai.Message, error) {
	var messages []ai.Message

	for rows.Next() {
		var role, content string
		var contentPartsJSON, toolCallsJSON, codeExecutionsJSON []byte
		var toolCallID, name, refusal, reasoning *string

		if err := rows.Scan(
			&role, &content, &contentPartsJSON, &toolCallsJSON,
			&toolCallID, &name, &refusal, &reasoning, &codeExecutionsJSON,
		); err != nil {
			return nil, fmt.Errorf("pgmemory: scan row: %w", err)
		}

		messages = append(messages, buildMessage(role, content, contentPartsJSON, toolCallsJSON, toolCallID, name, refusal, reasoning, codeExecutionsJSON))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgmemory: iterate rows: %w", err)
	}

	if messages == nil {
		return []ai.Message{}, nil
	}
	return messages, nil
}

// scanSingleMessage reads exactly one row from a pgx.Row and returns an
// ai.Message pointer. Returns (nil, nil) when the row is empty (pgx.ErrNoRows).
func scanSingleMessage(row pgx.Row) (*ai.Message, error) {
	var role, content string
	var contentPartsJSON, toolCallsJSON, codeExecutionsJSON []byte
	var toolCallID, name, refusal, reasoning *string

	err := row.Scan(
		&role, &content, &contentPartsJSON, &toolCallsJSON,
		&toolCallID, &name, &refusal, &reasoning, &codeExecutionsJSON,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("pgmemory: scan single row: %w", err)
	}

	msg := buildMessage(role, content, contentPartsJSON, toolCallsJSON, toolCallID, name, refusal, reasoning, codeExecutionsJSON)
	return &msg, nil
}

// buildMessage assembles an ai.Message from the raw column values scanned
// from a PostgreSQL row. Nullable TEXT columns are represented as *string;
// nil means SQL NULL.
func buildMessage(
	role, content string,
	contentPartsJSON, toolCallsJSON []byte,
	toolCallID, name, refusal, reasoning *string,
	codeExecutionsJSON []byte,
) ai.Message {
	msg := ai.Message{
		Role:       ai.MessageRole(role),
		Content:    content,
		ToolCallID: derefString(toolCallID),
		Name:       derefString(name),
		Refusal:    derefString(refusal),
		Reasoning:  derefString(reasoning),
	}

	if len(toolCallsJSON) > 0 {
		_ = json.Unmarshal(toolCallsJSON, &msg.ToolCalls)
	}
	if len(contentPartsJSON) > 0 {
		_ = json.Unmarshal(contentPartsJSON, &msg.ContentParts)
	}
	if len(codeExecutionsJSON) > 0 {
		_ = json.Unmarshal(codeExecutionsJSON, &msg.CodeExecutions)
	}

	return msg
}

// marshalNullableJSON marshals value to JSON, returning nil when the underlying
// slice is empty or nil. This maps Go zero-values to SQL NULL instead of
// storing empty JSON arrays ("[]") in JSONB columns.
func marshalNullableJSON(value any) ([]byte, error) {
	switch v := value.(type) {
	case []ai.ToolCall:
		if len(v) == 0 {
			return nil, nil
		}
	case []ai.ContentPart:
		if len(v) == 0 {
			return nil, nil
		}
	case []ai.CodeExecution:
		if len(v) == 0 {
			return nil, nil
		}
	}
	return json.Marshal(value)
}

// derefString safely dereferences a *string, returning "" for nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
