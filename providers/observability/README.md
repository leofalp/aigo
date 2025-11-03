# Observability

AIGO provides a comprehensive, lightweight observability system supporting tracing, metrics, and structured logging. The system is **optional** (zero overhead when disabled), **provider-agnostic**, and **OpenTelemetry-compatible**.

## Quick Start

### No Observability (Default)

```go
client, err := client.NewClient[string](
    openai.NewOpenAIProvider(),
)
// Zero overhead - no observability
```

### With Slog Observer

```go
import (
    "aigo/providers/observability/slogobs"
    "log/slog"
)

observer := slogobs.New(
    slogobs.WithFormat(slogobs.FormatCompact),
    slogobs.WithLevel(slog.LevelInfo),
)

client, err := client.NewClient[string](
    openai.NewOpenAIProvider(),
    client.WithObserver(observer),
)
```

## The Three Pillars

### 1. Tracing (Distributed Tracing)

Track request flow across components with spans:

```go
ctx, span := observer.StartSpan(ctx, "operation")
defer span.End()

span.SetAttributes(observability.String("key", "value"))
span.AddEvent("checkpoint")
```

### 2. Metrics (Counters & Histograms)

Track quantitative data:

```go
observer.Counter("requests").Add(ctx, 1)
observer.Histogram("duration").Record(ctx, 1.234)
```

### 3. Logging (Structured Logging)

Emit structured log messages at different levels:

```go
observer.Trace(ctx, "Protocol details", attrs...)  // Very verbose
observer.Debug(ctx, "Debug info", attrs...)         // Technical details
observer.Info(ctx, "Operation status", attrs...)    // High-level events
observer.Warn(ctx, "Warning", attrs...)             // Issues
observer.Error(ctx, "Error occurred", attrs...)     // Failures
```

## Logging Levels

| Level | Purpose | Use Cases | When to Enable |
|-------|---------|-----------|----------------|
| **TRACE** | Protocol/transport details | HTTP requests, raw payloads, network timing | Network debugging, API inspection |
| **DEBUG** | Technical debugging | Span events, memory ops, token usage, message content | Application debugging, flow analysis |
| **INFO** | Operational summaries | Start/completion, status, key metrics | Production monitoring, audit trails |
| **WARN/ERROR** | Problems | API errors, timeouts, failures | Always (production error tracking) |

## Available Implementations

### Slog Observer

Built-in implementation using Go's `log/slog`:

```go
import "aigo/providers/observability/slogobs"

// Three output formats:
// - FormatCompact: Single line with JSON (default)
// - FormatPretty:  Multi-line with emoji and tree
// - FormatJSON:    Standard JSON for log aggregation

observer := slogobs.New(
    slogobs.WithFormat(slogobs.FormatCompact),
    slogobs.WithLevel(slog.LevelInfo),
)

// Or use environment variables:
// AIGO_LOG_FORMAT=compact|pretty|json
// AIGO_LOG_LEVEL=TRACE|DEBUG|INFO|WARN|ERROR
observer := slogobs.New()
```

**Features**:
- Zero external dependencies
- Three output formats
- Environment-based configuration
- Auto-detected TTY colors
- Thread-safe

See `providers/observability/slogobs/` for detailed documentation.

### OpenTelemetry Observer (Future)

Native OpenTelemetry support planned.

## Span Propagation Pattern

### How It Works

1. **Layer 2 (Client)** creates a root span:
   ```go
   ctx, span := observer.StartSpan(ctx, "client.send_message")
   defer span.End()
   ```

2. **Layer 1 (Providers)** retrieve and enrich the span:
   ```go
   span := observability.SpanFromContext(ctx)
   if span != nil {
       span.SetAttributes(observability.String("llm.model", "gpt-4"))
       span.AddEvent("llm.request.start")
   }
   ```

3. **Context propagates** through the call stack, allowing all components to contribute.

### Key Principles

- **Single span per operation** - Client creates, providers enrich
- **Context-based propagation** - Span travels via `context.Context`
- **Nil-safe** - Always check `if span != nil`
- **Async-safe** - Spans are goroutine-safe

### Context Helpers

```go
// Store span in context
ctx = observability.ContextWithSpan(ctx, span)

// Retrieve span from context
span := observability.SpanFromContext(ctx)

// Store observer in context
ctx = observability.ContextWithObserver(ctx, observer)
```

## Semantic Conventions

All attributes follow standardized conventions (`providers/observability/semconv.go`):

### LLM Attributes
- `llm.provider` - Provider name (openai, anthropic)
- `llm.model` - Model identifier (gpt-4)
- `llm.endpoint` - API endpoint URL
- `llm.finish_reason` - Why generation stopped

### Token Attributes
- `llm.tokens.total` - Total tokens used
- `llm.tokens.prompt` - Input tokens
- `llm.tokens.completion` - Output tokens

### HTTP Attributes
- `http.method` - HTTP method
- `http.status_code` - Response status
- `http.request.body.size` - Request size
- `http.response.body.size` - Response size

### Memory/Client Attributes
- `memory.total_messages` - Messages in memory
- `client.prompt` - User input (truncated)
- `client.tools_count` - Available tools

### Event Names
- `llm.request.start/end` - LLM request lifecycle
- `memory.append` - Message appended
- `tool.execution.start/end` - Tool execution

### Span Names
- `client.send_message` - Client operation
- `llm.request` - LLM API request
- `tool.execution` - Tool execution

### Metric Names
- `aigo.client.request.count` - Request counter
- `aigo.client.request.duration` - Duration histogram
- `aigo.client.tokens.total` - Token counter

## Usage Example

```go
// Layer 2: Client creates span
func (c *Client[T]) SendMessage(ctx context.Context, prompt string) (*Response[T], error) {
    if c.observer != nil {
        ctx, span := c.observer.StartSpan(ctx, "client.send_message")
        defer span.End()
        
        span.SetAttributes(
            observability.String("client.prompt", truncate(prompt)),
        )
        
        // Propagate context
        ctx = observability.ContextWithSpan(ctx, span)
    }
    
    // Call provider (which enriches span)
    return c.provider.SendMessage(ctx, messages)
}

// Layer 1: Provider enriches span
func (p *OpenAIProvider) SendMessage(ctx context.Context, req Request) (*Response, error) {
    span := observability.SpanFromContext(ctx)
    if span != nil {
        span.SetAttributes(
            observability.String("llm.provider", "openai"),
            observability.String("llm.model", req.Model),
        )
        span.AddEvent("llm.request.start")
    }
    
    // Make API call
    resp, err := p.makeRequest(ctx, req)
    
    if span != nil {
        if err != nil {
            span.RecordError(err)
            span.SetStatus(observability.StatusError, "Request failed")
        } else {
            span.SetAttributes(
                observability.Int("llm.tokens.total", resp.Usage.TotalTokens),
            )
            span.SetStatus(observability.StatusOK, "")
        }
        span.AddEvent("llm.request.end")
    }
    
    return resp, err
}
```

## OpenTelemetry Mapping

AIGO's observability maps directly to OpenTelemetry:

| AIGO | OpenTelemetry |
|------|---------------|
| `Span` | `trace.Span` |
| `StartSpan()` | `tracer.Start()` |
| `Counter` | `Int64Counter` |
| `Histogram` | `Float64Histogram` |
| `StatusOK/Error` | `codes.Ok/Error` |

All semantic conventions follow [OTel standards](https://opentelemetry.io/docs/specs/semconv/).

## Best Practices

### 1. Always Check for Nil

```go
if observer != nil {
    observer.Info(ctx, "message")
}

span := observability.SpanFromContext(ctx)
if span != nil {
    span.SetAttributes(...)
}
```

### 2. Use Semantic Conventions

```go
// Good ✓
observability.String(observability.AttrLLMModel, model)

// Bad ✗
observability.String("model", model)
```

### 3. Truncate Long Strings

```go
truncated := observability.TruncateStringDefault(longText)
observer.Debug(ctx, "Content", observability.String("text", truncated))
```

### 4. Appropriate Log Levels

- **TRACE**: Protocol details only
- **DEBUG**: Technical debugging
- **INFO**: Business operations
- **WARN/ERROR**: Problems only

### 5. Set Span Status

```go
if err != nil {
    span.RecordError(err)
    span.SetStatus(observability.StatusError, "Failed")
} else {
    span.SetStatus(observability.StatusOK, "")
}
```

### 6. Propagate Context

Always pass context to downstream calls:

```go
ctx = observability.ContextWithSpan(ctx, span)
result := downstream(ctx, args)
```

## Performance

- **Nil observer**: Zero overhead
- **Enabled observer**: Minimal impact (<1% typically)
- **Thread-safe**: All operations are goroutine-safe
- **Async spans**: Safe for concurrent use

## Testing

### Mock Observer

```go
type mockObserver struct {
    spans []string
}

func (m *mockObserver) StartSpan(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span) {
    m.spans = append(m.spans, name)
    return ctx, &mockSpan{}
}
```

See `core/client/client_observability_test.go` for examples.

## Custom Observer Implementation

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
    Trace(ctx context.Context, msg string, attrs ...Attribute)
    Debug(ctx context.Context, msg string, attrs ...Attribute)
    Info(ctx context.Context, msg string, attrs ...Attribute)
    Warn(ctx context.Context, msg string, attrs ...Attribute)
    Error(ctx context.Context, msg string, attrs ...Attribute)
}
```

## Examples

- `examples/layer2/observability/` - Comprehensive demonstration
- `providers/observability/slogobs/` - Reference implementation
- `core/client/client_observability_test.go` - Unit tests

## See Also

- [Slog Observer Documentation](./slogobs/README.md)
- [Format Examples](./slogobs/FORMATS.md)
- [Semantic Conventions](./semconv.go)