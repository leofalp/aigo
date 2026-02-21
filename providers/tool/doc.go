// Package tool provides the foundational types and utilities for defining and
// executing tools that can be invoked by AI language models.
//
// A tool wraps a typed Go function together with its name, description, and
// auto-derived JSON schemas, making it ready for registration with any provider
// that implements the [ai.Provider] interface. The main entry point for creating
// tools is [NewTool]; option functions [WithDescription] and [WithMetrics] allow
// further configuration.
//
// The [Catalog] type offers a thread-safe registry for managing collections of
// tools; use [NewCatalog] or [NewCatalogWithTools] to create one.
package tool
