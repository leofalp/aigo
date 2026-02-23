// Package pgmemory provides a PostgreSQL-backed implementation of the
// [memory.Provider] interface for persisting chat message history across
// process restarts. Each [PgMemory] instance is scoped to a single session
// (conversation or thread), and uses pgx/v5 for efficient, pool-safe queries.
//
// This package lives in its own Go module to isolate the pgx dependency from
// the main aigo module, which is intentionally dependency-light.
//
// The main entry point is [New], which returns a ready-to-use [PgMemory]
// bound to a specific session. Use [EnsureSchema] during development to
// auto-create the required table; production deployments should manage
// schema migrations with dedicated tooling (goose, migrate, etc.).
package pgmemory
