# AGENTS.md

This file provides guidance for agentic coding assistants working with code in this repository.

## Project Overview

AIGO is a lightweight, modular Go framework for building AI applications with a 3-layer architecture:
- **Layer 1: providers/** - Pure I/O implementations (OpenAI, Gemini, memory stores, tools, observability)
- **Layer 2: core/** - Shared business logic (Client orchestration, cost tracking, parsing)
- **Layer 3: patterns/** - High-level AI patterns (ReAct with type-safe structured output)

Each layer is independent. Layer 1 can be used standalone; Layer 2 requires Layer 1; Layer 3 can use either.

## Build, Test, and Lint Commands

```bash
# Run all unit tests
go test ./...

# Run with race detector (required before PR)
go test -race ./...

# Run single test function
go test ./path/to/package -run TestFunctionName -v

# Run tests for specific package
go test ./providers/ai/openai/...

# Run integration tests (requires internet, calls real APIs)
go test -tags=integration ./...

# Generate coverage report
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Lint code
golangci-lint run

# Format code (required before committing)
go fmt ./...
gofmt -s -w .

# Build all packages
go build ./...
```

## Architecture

```
aigo/
├── core/
│   ├── client/       # Main orchestrator - stateful/stateless modes, tool execution
│   ├── cost/         # Cost tracking (model, tool, compute costs)
│   └── parse/        # JSON extraction and type-safe parsing
├── providers/
│   ├── ai/           # AI providers (openai/, gemini/)
│   ├── memory/       # Conversation persistence (inmemory/)
│   ├── tool/         # Tool interface and implementations
│   └── observability/# slog-based structured logging
├── patterns/
│   └── react/        # Type-safe ReAct[T] with automatic tool execution loops
├── internal/
│   ├── utils/        # HTTP, timer, string, pointer helpers
│   └── jsonschema/   # JSON schema generation from Go types
└── examples/         # Working examples for each layer (layer1/, layer2/, layer3/)
```

### Key Entry Points
- `core/client/client.go` - Main Client orchestrator
- `patterns/react/react.go` - ReAct[T] agent pattern
- `providers/tool/tool.go` - Tool interface and generic Tool[I,O]

## Code Conventions

### Import Ordering
1. Standard library (alphabetically)
2. External packages (alphabetically)
3. Internal packages (alphabetically)

### Naming
- **Exported**: `PascalCase` (`Client`, `SendMessage`)
- **Private**: `camelCase` (`mockProvider`, `sendMessage`)
- **Constants**: `PascalCase` (`EnvDefaultModel`, `MaxIterations`)
- **Interfaces**: Simple, action-oriented (`Provider`, `Tool`)
- **Constructors**: `New()` default, `NewTypeName()` specific

### Error Handling
```go
// Always wrap errors with %w
if err != nil {
    return fmt.Errorf("failed to process input: %w", err)
}

// Use defer for cleanup
defer utils.CloseWithLog(resp.Body)
```

### Function Signatures
```go
// Context always first parameter
func SendMessage(ctx context.Context, request ChatRequest) (*ChatResponse, error)

// Return (T, error) for operations that may fail
func Parse[T any](content string) (T, error)

// Use generics for type-safe outputs
func NewStructured[T any](provider ai.Provider, opts ...Option) (*Client[T], error)
```

### Common Patterns

**Option Pattern:**
```go
type ClientOption func(*Client)

func WithMemory(memoryProvider memory.Provider) ClientOption {
    return func(c *Client) { c.memoryProvider = memoryProvider }
}
```

**Fluent Interface:**
```go
func (p *OpenAIProvider) WithAPIKey(apiKey string) ai.Provider {
    p.apiKey = apiKey
    return p
}
```

### Reusable Utilities

**ALWAYS check `internal/utils/` before creating new utilities:**
- `CloseWithLog(io.Closer)` - Safe closer with logging
- `DoPostSync[T]()` - HTTP POST with observability
- `TruncateString()` - String manipulation
- `NewTimer()` - Timing utilities
- `ToPointer[T]()` - Value to pointer conversion

Add to `internal/utils/` only when used in 2+ packages and has no business logic.

### Documentation
- All exported types/functions MUST have godoc comments
- Full sentences with proper punctuation
- Include examples for complex types

## Testing

**Unit Tests** (`*_test.go`):
- Work offline (mock all external dependencies)
- Use `httptest.Server` for HTTP mocking
- Descriptive names: `TestSendMessage_WithValidRequest_ReturnsResponse`

**Integration Tests** (`*_integration_test.go`):
- Must have `//go:build integration` on first line
- Call real external APIs
- NOT run in CI/CD

## Configuration

Required environment variables:
```bash
OPENAI_API_KEY=your-api-key
AIGO_DEFAULT_LLM_MODEL=your-model-name
```

Optional cost tracking, logging, and tool-specific keys documented in `.env.example`.

## CI/CD

- Tests run on Go 1.24 and 1.25
- Command: `go test -race -coverprofile=coverage.out ./...`
- Integration tests NOT run in CI

## LLM Documentation Files

`llms.txt` and `llms-full.txt` at the repo root are machine-readable documentation files following the [llms.txt specification](https://llmstxt.org/). They provide LLM-friendly navigation and comprehensive API reference.

**Keep these files updated when making substantial library changes**, including:
- Adding, removing, or renaming exported types, functions, or methods
- Adding new packages (`providers/ai/`, `providers/tool/`, `patterns/`, `core/`)
- Changing constructor signatures or option functions
- Adding or removing built-in tools
- Updating environment variable names or semantics

`llms.txt` is the concise navigation index (under 200 lines). `llms-full.txt` is the comprehensive reference including README, architecture docs, exported API signatures, and key examples.

## Changelog

`CHANGELOG.md` at the repo root tracks all notable changes per release following the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format and [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**Update `CHANGELOG.md` on every release.** When preparing a new version:

1. Add a new `## [vX.Y.Z] - YYYY-MM-DD` section at the top (below the header), keeping previous versions below it.
2. Organize changes under these subsections (omit empty ones):
   - `### New Features` -- New capabilities, tools, providers, or patterns.
   - `### Refactoring & Improvements` -- Non-breaking enhancements, renames, documentation improvements.
   - `### Bug Fixes` -- Defect corrections.
   - `### Breaking Changes` -- API changes that require consumer updates.
   - `### Testing` -- Significant test additions or improvements.
   - `### Build & CI` -- Dependency upgrades, CI changes, tooling updates.
3. Each entry should be a concise bullet point starting with a **bolded name** followed by a short description.
4. Add a comparison link at the bottom of the file: `[vX.Y.Z]: https://github.com/leofalp/aigo/compare/vPREV...vX.Y.Z`.

## Pre-Commit Checklist

1. All unit tests pass: `go test -race ./...`
2. No linting errors: `golangci-lint run`
3. Code formatted: `go fmt ./... && gofmt -s -w .`
4. Checked `internal/utils/` for existing utilities
5. All exported types/functions have godoc comments
6. If substantial library changes were made: update `llms.txt` and `llms-full.txt`
7. If preparing a release: update `CHANGELOG.md` with the new version section
