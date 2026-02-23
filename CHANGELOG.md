# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### New Features

- **Middleware system** (`core/client/middleware/`) -- Composable chain for LLM calls with built-in support for **Retry** (backoff/jitter), **Timeout** (deadlines), **Logging** (slog), and **Observability** (tracing/metrics). Supports both sync and stream paths.
- **Streaming ReAct** (`patterns/react/`) -- Real-time event delivery for the ReAct agent via `ExecuteStream`. Yields reasoning, tool calls, and results through an iterator.
- **PostgreSQL memory provider** (`providers/memory/pgmemory/`) -- Persistent chat history using `pgx/v5` with session isolation and multimodal content support.

### Refactoring & Improvements

- **Centralized Observability** -- Moved inline logic into `ObservabilityMiddleware`, adding full support for streaming spans and metrics.
- **Memory Provider Interface** -- Updated methods to accept `context.Context` and return `error` to support database-backed implementations.

### Breaking Changes

- **memory.Provider** -- Interface updated: all methods now take `context.Context`; read methods now return an `error`; `Clear` renamed to `ClearMessages`.

### Testing

- Added comprehensive unit tests for Retry, Timeout, Logging, and Observability middlewares covering success/error/streaming/context-propagation scenarios.

## [v0.3.0] - 2025-02-22

### New Features

- **Streaming support** -- Native SSE streaming for OpenAI and Gemini with new `StreamProvider` interface, `ChatStream`/`StreamDelta` types, `DoStreamingPostSync` helper, and SSE scanner with 1MB buffer.
- **Graph pattern** (`patterns/graph/`) -- DAG pattern for orchestrating multi-step LLM workflows with fluent builder, structural validation (Kahn's algorithm for cycle detection), parallel execution per topological level, conditional edges, pluggable `StateProvider`, fail-fast/continue-on-error strategies, per-node and per-graph timeouts, and full observability integration.
- **Exa tool** (`providers/tool/exa/`) -- Full Exa AI integration: search, content retrieval, find-similar, and answer.
- **Tavily tool** (`providers/tool/tavily/`) -- Full Tavily integration: web search and extract.
- **Multimodal support** -- New content types for audio, video, documents, and images in `providers/ai/models.go` with dedicated constructors. Gemini code execution support.
- **Core Overview** (`core/overview/`) -- Moved `patterns.Overview` to `core/overview` with JSON tags and generic `StructuredOverview[T]`.
- **Gemini 3.1 Pro Preview** -- New model with pricing in the Gemini catalog.
- **Tiered pricing** -- Input/output cost calculations with configurable threshold tiers.
- **Calculator tool** -- New tool with comprehensive tests.

### Refactoring & Improvements

- Renamed `parse.JSONRepair` to `Repair` for compatibility with the newer library version.
- Added `doc.go` for nearly every package. Improved godoc comments across the codebase.
- Standardized on US English spelling across all packages.
- Memory safety improvements: SSE scanner buffer raised to 1MB, response body reads capped at 10MB, rune-based slicing for UTF-8 in Gemini streaming deltas.
- New attributes and methods in the observability provider, simplified `semconv`.

### Testing

- Added +10,000 lines of tests covering streaming, graph, Exa, Tavily, BraveSearch, DuckDuckGo, calculator, strings, timer, SSE scanner, and HTTP stream. Integration tests for Gemini, OpenAI, Exa, Tavily, and webfetch.

### Build & CI

- Go upgraded to 1.25 (CI runs on 1.24 + 1.25).
- Added `llms.txt` and `llms-full.txt` (machine-readable documentation).

## [v0.2.0] - 2025-02-03

### New Features

- **Gemini AI provider** -- Full API support for Google's Gemini models.
- **Google Search grounding** -- Grounding with citations support for Gemini.
- **SiteDataExtractor tool** -- New tool for extracting structured data from websites.
- **WebFetch improvements** -- Enhanced reliability and functionality of the WebFetch tool.

## [v0.1.0] - 2024-12-05

### New Features

- **Initial public release** of AIGO, a Go framework for building AI applications.
- **3-layer architecture** -- Layer 1 (Providers), Layer 2 (Core), Layer 3 (Patterns).
- **Type-safe structured output** with Go generics.
- **OpenAI-compatible provider** as the first AI provider.
- **Built-in cost tracking** for model usage.
- **Observability support** -- Tracing, metrics, and logging.
- **ReAct agent pattern** -- Ready-to-use agentic pattern with automatic tool execution loops.
- **Minimal dependencies** -- Lightweight framework with few external requirements.

[v0.3.0]: https://github.com/leofalp/aigo/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/leofalp/aigo/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/leofalp/aigo/commits/v0.1.0
