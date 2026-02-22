// Package inmemory provides a concurrency-safe, slice-backed implementation
// of the [memory.Provider] interface for storing chat message history in process memory.
// It is designed for single-process use cases where persistence across restarts is not required.
// The main entry point is [New], which returns a ready-to-use [ArrayMemory] instance.
package inmemory
