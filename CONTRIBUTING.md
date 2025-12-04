# Contributing to AIGO

Thank you for your interest in contributing to AIGO! We welcome contributions from the community.

## How to Contribute

1. **Fork** the repository
2. **Create** a new branch (`git checkout -b feature/your-feature`)
3. **Make** your changes
4. **Test** your changes locally (see below)
5. **Commit** your changes (`git commit -am 'Add new feature'`)
6. **Push** to your branch (`git push origin feature/your-feature`)
7. **Open** a Pull Request

## Running Tests Locally

Before pushing your commits, always run the full test suite locally:

```bash
# Run unit tests (default - no external API calls, always run in CI)
go test ./...

# Run unit tests with race detector (recommended before PR)
go test -race ./...

# Run tests with coverage report
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out  # View coverage in browser

# Run integration tests (requires internet, calls real APIs)
go test -tags=integration ./...

# Run ALL tests (unit + integration)
go test -tags=integration -race ./...

# Run linting
golangci-lint run

# Format your code
go fmt ./...
gofmt -s -w .
```

Make sure all tests pass and there are no linting errors before committing.

## Testing Guidelines

### Unit Tests (Required for CI/CD)

**File naming**: `*_test.go`

**Characteristics**:
- Must work **offline** (no external network calls)
- Mock all external dependencies (HTTP servers, databases, APIs)
- Fast execution (milliseconds)
- **Always run in CI/CD**
- Use `httptest.Server` for HTTP mocking

### Integration Tests (Manual Verification)

**File naming**: `*_integration_test.go`

**Build tag**: Must start with `//go:build integration` on the first line

**Characteristics**:
- Call **real external APIs**
- Require internet connection
- Slower execution (seconds)
- **NOT run in CI/CD** (only manual)
- Used for pre-release verification

**Running integration tests**:
```bash
# Run integration tests for specific package
go test -tags=integration ./providers/tool/mypackage/...

# Run all integration tests
go test -tags=integration ./...
```

### Test Coverage

- Write tests for **all** new functionality
- Aim for **>70%** coverage for core packages
- Run `go test ./... -cover` to verify coverage
- All tests must pass before submitting a PR

## Code Style Guidelines

### Idiomatic Go

- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Run `go fmt` and `gofmt -s` before committing
- Use `golangci-lint` for linting (must pass with 0 issues)

### Dependencies

- **Minimize external dependencies**
- Each new dependency must be justified
- Place provider-specific code in separate packages to leverage Go's module graph pruning
- Core packages should rely primarily on the standard library

### Reusable Code in `internal/`

**Before creating a new utility function, always check if it already exists in `internal/`**

The `internal/` directory contains reusable utilities:
- `internal/utils/` - Common helper functions (HTTP, strings, timers, pointers)
- `internal/jsonschema/` - JSON Schema generation

**Guidelines**:
1. Check `internal/utils/` before writing common helpers
2. If you create a generic function that could be reused, place it in `internal/utils/`
3. Use godoc comments for all exported functions in `internal/`
4. Keep `internal/` functions focused and well-tested

**Examples of utilities in `internal/utils/`**:
- `CloseWithLog(io.Closer)` - Safe closer with logging
- `DoPostSync[T]()` - HTTP POST with observability
- `TruncateString()` - String manipulation
- `NewTimer()` - Timing utilities

**When to add to `internal/utils/`**:
- ✅ Generic function used in 2+ packages
- ✅ Common pattern that could benefit others
- ✅ Utility with no business logic
- ❌ Provider-specific logic
- ❌ One-off helper functions

## Documentation

### Godoc Comments

**All exported types and functions must have godoc comments**:

```go
// MyFunction performs a specific operation and returns the result.
// It returns an error if the operation fails.
func MyFunction(input string) (string, error) {
    // implementation
}
```

### Examples

Keep examples in `examples/` directory up to date when:
- Adding new features
- Changing public APIs
- Introducing new patterns

### README Updates

Update relevant README files when:
- Adding new providers
- Changing configuration options
- Introducing breaking changes

## Pull Request Process

### Before Submitting

1. ✅ All unit tests pass (`go test ./...`)
2. ✅ No linting errors (`golangci-lint run`)
3. ✅ Code is formatted (`go fmt ./...`)
4. ✅ Integration tests pass (if applicable)
5. ✅ Documentation is updated
6. ✅ Commits are clean and focused

### PR Description

Provide a clear description including:
- What problem does this solve?
- What changes were made?
- Are there breaking changes?
- How to test the changes?

### Review Process

- Keep commits clean and focused on a single change
- Respond to review comments promptly
- Be open to feedback and suggestions
- Squash commits before merge if requested

## Development Workflow

### Typical Development Cycle

```bash
# 1. Create feature branch
git checkout -b feature/my-feature

# 2. Make changes
vim some_file.go

# 3. Run tests frequently during development
go test ./path/to/package/...

# 4. Before committing
go fmt ./...
golangci-lint run
go test -race ./...

# 5. Commit
git add .
git commit -m "Add my feature"

# 6. Push and create PR
git push origin feature/my-feature
```

### Pre-Release Checklist

Before creating a release tag:

```bash
# 1. Run all unit tests with race detector
go test -race ./...

# 2. Run integration tests
go test -tags=integration ./...

# 3. Run linting
golangci-lint run

# 4. Check coverage
go test -coverprofile=coverage.out ./...

# 5. Build all packages
go build ./...
```

## Common Patterns

### Error Handling

```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to process input: %w", err)
}
```

### Resource Cleanup

```go
// Use defer with proper error handling
resp, err := http.Get(url)
if err != nil {
    return err
}
defer utils.CloseWithLog(resp.Body)
```

### Context Propagation

```go
// Always pass context as first parameter
func MyFunction(ctx context.Context, input string) error {
    // Use ctx for cancellation, timeouts, values
}
```

## Questions or Issues?

- 💬 Open a [GitHub Discussion](https://github.com/leofalp/aigo/discussions) for questions
- 🐛 Open a [GitHub Issue](https://github.com/leofalp/aigo/issues) for bugs
- 📧 Contact maintainers for security issues

---

By contributing, you agree that your contributions will be licensed under the MIT License.