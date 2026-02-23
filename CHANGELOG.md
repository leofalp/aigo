# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### New Features

- **Middleware chain** (`core/client/middleware/`) -- Composable middleware framework for the `Client` with `SendFunc`, `StreamFunc`, `Middleware`, `StreamMiddleware`, and `MiddlewareConfig` types. Middlewares are applied outermost-first and can intercept both synchronous and streaming provider calls. Use `WithMiddleware(...)` to register them.
- **Retry middleware** (`NewRetryMiddleware`) -- Exponential backoff with configurable jitter, max retries, and a pluggable retryable predicate. Defaults to retrying on HTTP 429/500/502/503/529. Streaming calls bypass retry (mid-stream errors cannot be transparently replayed). Sentinel error `ErrRetryExhausted` allows callers to distinguish exhaustion from other failures.
- **Timeout middleware** (`NewTimeoutMiddleware`) -- Per-request deadline enforcement for both send and stream calls. For streaming, the timeout governs the full stream lifetime (not just time-to-first-byte), and the context cancel is deferred until the stream is consumed or abandoned.
- **Logging middleware** (`NewLoggingMiddleware`) -- Structured `slog` log entries at configurable verbosity levels (`LogLevelMinimal`, `LogLevelStandard`, `LogLevelVerbose`). Covers both send and stream paths; stream completion is logged when the iterator is fully drained.
- **Observability middleware** (`NewObservabilityMiddleware`) -- Distributed tracing spans, request/token counters, and duration histograms for every LLM call including streaming. Auto-prepended as the outermost middleware by `New()` when `WithObserver()` is provided, replacing the previous inline observability in `client.go`.

### Refactoring & Improvements

- Moved client-level observability from inline code in `SendMessage`/`ContinueConversation` into `ObservabilityMiddleware`, centralising span/metric/log logic and adding streaming observability that was previously missing.
- Streaming observability now records spans, counters, and histograms when the `ChatStream` iterator is fully consumed, errored, or abandoned, matching the behaviour of the synchronous path.

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
