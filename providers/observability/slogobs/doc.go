// Package slogobs provides an observability.Provider implementation backed by
// Go's standard library log/slog package.
// It supports structured tracing, in-memory metrics, and levelled logging
// through a configurable slog.Handler that can emit compact, pretty, or JSON output.
// The main entry point is [New]; output format and log level can be tuned with
// [WithFormat], [WithLevel], [WithOutput], [WithColors], and [WithLogger].
package slogobs
