package pgmemory

import (
	"context"
	"fmt"
)

// createTableSQL is the DDL statement that creates the aigo_messages table.
// All ai.Message fields are persisted to guarantee round-trip fidelity:
// messages read back from PostgreSQL are identical to the originals.
//
// The seq column (BIGSERIAL) provides monotonic ordering within a session,
// avoiding timestamp collisions from rapid-fire messages within the same
// microsecond.
const createTableSQL = `CREATE TABLE IF NOT EXISTS %s (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    seq             BIGSERIAL NOT NULL,
    session_id      TEXT NOT NULL,
    role            TEXT NOT NULL,
    content         TEXT NOT NULL DEFAULT '',
    content_parts   JSONB,
    tool_calls      JSONB,
    tool_call_id    TEXT,
    name            TEXT,
    refusal         TEXT,
    reasoning       TEXT,
    code_executions JSONB,
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

// createSessionSeqIndexSQL creates the primary lookup index: all messages
// for a session ordered by insertion sequence.
const createSessionSeqIndexSQL = `CREATE INDEX IF NOT EXISTS idx_%s_session_seq
    ON %s (session_id, seq)`

// createSessionRoleIndexSQL creates an index for role-based filtering
// within a session (used by FilterByRole).
const createSessionRoleIndexSQL = `CREATE INDEX IF NOT EXISTS idx_%s_session_role
    ON %s (session_id, role)`

// EnsureSchema creates the aigo_messages table and its indexes if they do
// not already exist. This is a convenience helper for development and
// prototyping; production deployments should use proper migration tooling
// (goose, golang-migrate, etc.) to manage schema changes.
func (m *PgMemory) EnsureSchema(ctx context.Context) error {
	tableSQL := fmt.Sprintf(createTableSQL, m.tableName)
	if _, err := m.db.Exec(ctx, tableSQL); err != nil {
		return fmt.Errorf("pgmemory: create table: %w", err)
	}

	seqIdxSQL := fmt.Sprintf(createSessionSeqIndexSQL, m.tableName, m.tableName)
	if _, err := m.db.Exec(ctx, seqIdxSQL); err != nil {
		return fmt.Errorf("pgmemory: create session_seq index: %w", err)
	}

	roleIdxSQL := fmt.Sprintf(createSessionRoleIndexSQL, m.tableName, m.tableName)
	if _, err := m.db.Exec(ctx, roleIdxSQL); err != nil {
		return fmt.Errorf("pgmemory: create session_role index: %w", err)
	}

	return nil
}
