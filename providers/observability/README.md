# Observability

The observability package provides a generic interface for observing your AI applications through **tracing**, **metrics**, and **logging**. It follows the same layered architecture as the rest of `aigo` - you choose which implementation to use.

## Philosophy

- **No lock-in**: Generic interface with pluggable implementations
- **Zero overhead by default**: Observer is `nil` if not specified (literally zero cost)
- **Thread-safe**: All implementations are safe for concurrent use
- **Composable**: Works seamlessly with Layer 2 (Core) and Layer 3 (Patterns)

## The Three Pillars

### 1. Tracing (Distributed Tracing)
Track execution flow and performance with spans:
- Start/end spans to measure operation duration
- Add attributes for context (model, tokens, etc.)
- Record errors and events
- Nest spans for hierarchical tracing

### 2. Metrics (Counters & Histograms)
Collect quantitative data about your application:
- **Counters**: Monotonically increasing values (request count, token usage)
- **Histograms**: Distribution of values (latencies, response sizes)

### 3. Logging (Structured Logging)
Emit structured log messages at different levels:
- Debug, Info, Warn, Error levels
- Attach key-value attributes for context
- Context-aware (can include trace IDs, etc.)

## Available Implementations

### 1. Slog Observer
**Location**: `providers/observability/slog`

Uses Go's standard library `log/slog` for logging-based observability.

```go
import (
    "aigo/providers/observability/slog"
    "log/slog"
    "os"
)

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
observer := slog.New(logger)
```

**Performance**: ~124ns for spans, ~22ns for metrics, ~558ns for logging

**Features**:
- Uses standard library (no external dependencies)
- Thread-safe metric storage
- Respects log levels
- JSON or text output formats

### 2. OpenTelemetry Observer (Future)
**Location**: `providers/observability/otel` (not yet implemented)

Full OpenTelemetry integration for enterprise observability.

## Usage

### Basic Setup (No Observability)

```go
import (
    "aigo/core/client"
    "aigo/providers/ai/openai"
)

// Default behavior - nil observer (zero overhead)
client := client.NewClient[string](
    openai.NewAPIProvider(),
    client.WithDefaultModel("gpt-4"),
)
// client.observer is nil - no observability overhead at all
```

### With Slog Observer

```go
import (
    "aigo/core/client"
    "aigo/providers/ai/openai"
    "aigo/providers/observability/slog"
    logslog "log/slog"
    "os"
)

// Create a logger
logger := logslog.New(logslog.NewJSONHandler(os.Stdout, &logslog.HandlerOptions{
    Level: logslog.LevelInfo,
}))

// Create client with observability
client := client.NewClient[string](
    openai.NewOpenAIProvider(),
    client.WithDefaultModel("gpt-4"),
    client.WithObserver(slog.New(logger)),
)

resp, err := client.SendMessage("Hello!")
```

### Custom Observer

Implement the `observability.Provider` interface:

```go
type Provider interface {
    Tracer
    Metrics
    Logger
}

type Tracer interface {
    StartSpan(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span)
}

type Metrics interface {
    Counter(name string) Counter
    Histogram(name string) Histogram
}

type Logger interface {
    Debug(ctx context.Context, msg string, attrs ...Attribute)
    Info(ctx context.Context, msg string, attrs ...Attribute)
    Warn(ctx context.Context, msg string, attrs ...Attribute)
    Error(ctx context.Context, msg string, attrs ...Attribute)
}
```

## Standard Metrics

The Core Client (Layer 2) emits these standard metrics:

### Counters
- `aigo.client.request.count` - Total number of requests
  - Attributes: `status` (success/error), `model`
- `aigo.client.tokens.total` - Total tokens used
  - Attributes: `model`
- `aigo.client.tokens.prompt` - Prompt tokens used
  - Attributes: `model`
- `aigo.client.tokens.completion` - Completion tokens used
  - Attributes: `model`

### Histograms
- `aigo.client.request.duration` - Request duration in seconds
  - Attributes: `model`

## Standard Spans

The Core Client creates these spans:

- `client.SendMessage` - Main request span
  - Attributes: `model`, `prompt`, `tokens.total`, `tokens.prompt`, `tokens.completion`
  - Events: errors, tool calls

## Attributes

Helper functions to create typed attributes:

```go
import "aigo/providers/observability"

observability.String("key", "value")
observability.Int("count", 42)
observability.Int64("big", 9223372036854775807)
observability.Float64("rate", 3.14)
observability.Bool("flag", true)
observability.Duration("latency", 5*time.Second)
observability.Error(err)
```

## Examples

### Example 1: Debug-level Tracing

```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

client := client.NewClient[string](
    openai.NewOpenAIProvider(),
    client.WithDefaultModel("gpt-4"),
    client.WithObserver(slog.New(logger)),
)

resp, _ := client.SendMessage("What is AI?")
// Outputs:
// level=DEBUG msg="Span started" span=client.SendMessage event=span.start model=gpt-4
// level=DEBUG msg="Sending message to LLM" model=gpt-4 tools_count=0
// level=INFO msg="Message sent successfully" finish_reason=stop duration=1.234s tool_calls=0
// level=DEBUG msg="Span ended" span=client.SendMessage event=span.end duration=1.234s
```

### Example 2: Info-level Logging Only

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

client := client.NewClient[string](
    openai.NewOpenAIProvider(),
    client.WithDefaultModel("gpt-4"),
    client.WithObserver(slog.New(logger)),
)

resp, _ := client.SendMessage("Tell me a joke")
// Outputs (JSON):
// {"level":"INFO","msg":"Message sent successfully","finish_reason":"stop","duration":1.5,"tool_calls":0}
```

### Example 3: Error Tracking

```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelError,
}))

client := client.NewClient[string](
    openai.NewOpenAIProvider().WithAPIKey("invalid"),
    client.WithDefaultModel("gpt-4"),
    client.WithObserver(slog.New(logger)),
)

_, err := client.SendMessage("Hello")
// Outputs:
// level=ERROR msg="Span error" span=client.SendMessage event=error error="authentication failed"
// level=ERROR msg="Failed to send message to LLM" error="authentication failed" duration=0.123s
```

## Testing

Mock the observer for tests:

```go
type mockObserver struct{}

func (m *mockObserver) StartSpan(ctx context.Context, name string, attrs ...observability.Attribute) (context.Context, observability.Span) {
    return ctx, &mockSpan{}
}

func (m *mockObserver) Counter(name string) observability.Counter {
    return &mockCounter{}
}

// ... implement other methods

// Use in tests
client := client.NewClient[string](provider, client.WithObserver(&mockObserver{}))
```

## Performance

Benchmark results (Apple M1 Pro):

| Implementation | StartSpan | Counter | Logging |
|----------------|-----------|---------|---------|
| Nil (default)  | 0 ns     | 0 ns    | 0 ns    |
| Slog           | 124 ns   | 22 ns   | 558 ns  |

**Recommendation**: Use `nil` (default) for production if you don't need observability, slog for development/debugging, and OpenTelemetry (future) for enterprise monitoring.

## Best Practices

1. **Start with nil**: Don't specify an observer unless you need it (zero overhead)
2. **Use appropriate log levels**: Debug for traces, Info for important events, Error for failures
3. **Add context with attributes**: Include model, tokens, and other relevant data
4. **Measure what matters**: Focus on request duration, token usage, and error rates
5. **Filter by level**: Use `slog.LevelInfo` or higher in production to reduce overhead

## Thread Safety

All implementations are thread-safe:
- **Nil**: No operations performed, inherently safe
- **Slog**: Uses `sync.RWMutex` for metric storage, `slog.Logger` is thread-safe

## Future Enhancements

- [ ] OpenTelemetry integration
- [ ] Gauge metrics
- [ ] Distributed tracing with trace/span IDs
- [ ] Sampling strategies
- [ ] Export to Prometheus, DataDog, etc.
- [ ] Semantic conventions for AI operations

## See Also

- [ARCHITECTURE.md](../../ARCHITECTURE.md) - Overall project architecture
- [examples/observability/main.go](../../examples/observability/main.go) - Complete examples
- Go's official [log/slog](https://pkg.go.dev/log/slog) documentation