# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.4.1] - 2025-02-25

### Bug Fixes

- **Gemini streaming** -- Fixed text truncation in `streamGenerateContent` responses. The streaming parser incorrectly assumed Gemini returned cumulative text in each SSE chunk and applied rune-based slicing to compute deltas. In reality, Gemini returns deltas (only the new text) in each chunk — identical to OpenAI and Anthropic. The slicing logic mangled multi-chunk responses (e.g., "Hello world!" became "Hellod").

### Testing

- **Gemini streaming unit tests** -- Updated mock SSE data from cumulative to delta format, matching the actual Gemini API behaviour.
- **Gemini streaming integration test** -- Added `TestGeminiStreamDeltaIntegrity_Integration` regression test that cross-validates streamed delta concatenation (`Iter()`), collected output (`Collect()`), and non-streaming baseline (`SendMessage`) to prevent future truncation regressions.

## [v0.4.0] - 2025-02-25

### New Features

- **Anthropic AI provider** (`providers/ai/anthropic/`) -- Full support for Claude Messages API (Claude 3.5/3.7) with streaming, tool calling, vision (multimodal input), extended thinking, prompt caching, and configurable beta features via `Capabilities` and `WithCapabilities`. Reads `ANTHROPIC_API_KEY` / `ANTHROPIC_API_BASE_URL` from env.
- **Middleware system** (`core/client/middleware/`) -- Composable chain for LLM calls with built-in support for **Retry** (exponential backoff with jitter), **Timeout** (context deadlines), **Logging** (structured logging with slog), and **Observability** (distributed tracing and metrics). Supports both sync and stream execution paths.
- **Streaming ReAct & Graph** (`patterns/react/`, `patterns/graph/`) -- Native streaming implementation for ReAct agents and execution graphs via `ExecuteStream`. Real-time event delivery yields reasoning steps, tool calls, and results through an iterator with async handling of tool execution loops.
- **PostgreSQL memory provider** (`providers/memory/pgmemory/`) -- Sub-module implementing the `memory.Provider` interface using `pgx/v5` for persistent chat history storage. Features session isolation, multimodal content support, and full CRUD operations on conversation history.

### Breaking Changes

- **memory.Provider** -- Interface updated: all methods (`Count`, `AllMessages`, `LastMessages`, `PopLastMessage`, `FilterByRole`) now accept `context.Context` as first parameter and return an `error`. The `Clear` method has been renamed to `ClearMessages`. Custom implementations must be updated accordingly.

### Refactoring & Improvements

- **Client Orchestrator** -- Migrated observability logic from inline implementation into the new `ObservabilityMiddleware`, providing unified tracing and metrics for both sync and streaming LLM calls.
- **Built-in Tools** -- Enhanced documentation and inline comments for Tavily, Exa, and BraveSearch tool providers, improving developer experience and API clarity.
- **Documentation** -- Updated `AGENTS.md`, `llms.txt`, and `llms-full.txt` with new architectural patterns, streaming capabilities, and sub-module release processes.

### Testing

- **Coverage** -- Increased global test coverage to **88.7%** with comprehensive unit tests across all new features.
- **Integration Tests** -- Introduced `requireAPIKey` helper that forces explicit test failure (`t.Fatal`) when credentials are missing, preventing silent skips and ensuring proper CI validation.
- **Sub-module Testing** -- Added complete test suite for `pgmemory` provider including unit tests (with mocks) and integration tests (with Docker-based PostgreSQL containers).
- **Middleware Tests** -- Added comprehensive test coverage for Retry, Timeout, Logging, and Observability middlewares, validating success/error/streaming/context-propagation scenarios.

### Build & CI

- **Go Workspace** -- Configured `.github/workflows/ci.yml` to initialize a Go workspace (`go work`) during CI runs. This enables testing the `pgmemory` sub-module against local changes in the main module before release, ensuring compatibility.
- **Dependency Management** -- Removed `replace` directives from `pgmemory`'s `go.mod` to guarantee compatibility with external consumers and proper module resolution.

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

[v0.4.1]: https://github.com/leofalp/aigo/compare/v0.4.0...v0.4.1
[v0.4.0]: https://github.com/leofalp/aigo/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/leofalp/aigo/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/leofalp/aigo/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/leofalp/aigo/commits/v0.1.0
