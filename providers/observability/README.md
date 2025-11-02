# Observability

The observability package provides a comprehensive system for monitoring AI applications through **tracing**, **metrics**, and **logging**. It follows aigo's layered architecture with pluggable implementations and zero overhead by default.

## Philosophy

- **No lock-in**: Generic interface with pluggable implementations
- **Zero overhead by default**: Observer is `nil` if not specified (literally zero cost)
- **Thread-safe**: All implementations are safe for concurrent use
- **Composable**: Works seamlessly across all layers
- **Context-based propagation**: Spans flow through layers via `context.Context`

## Table of Contents

- [The Three Pillars](#the-three-pillars)
- [Available Implementations](#available-implementations)
- [Quick Start](#quick-start)
- [Span Propagation Pattern](#span-propagation-pattern)
- [Semantic Conventions](#semantic-conventions)
- [Standard Metrics & Spans](#standard-metrics--spans)
- [Usage Examples](#usage-examples)
- [Best Practices](#best-practices)
- [Performance](#performance)
- [Testing](#testing)

## The Three Pillars

### 1. Tracing (Distributed Tracing)
Track execution flow and performance with spans:
- Start/end spans to measure operation duration
- Add attributes for context (model, tokens, etc.)
- Record errors and events
- Nest spans for hierarchical tracing
- **Propagate spans via context** for fine-grained visibility

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

Full OpenTelemetry integration for enterprise observability with exporters to Jaeger, Prometheus, DataDog, etc.

## Quick Start

### No Observability (Default - Zero Overhead)

```go
import (
    "aigo/core/client"
    "aigo/providers/ai/openai"
)

// Default behavior - nil observer (zero overhead)
client := client.NewClient[string](
    openai.NewOpenAIProvider(),
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

## Span Propagation Pattern

### Overview

The span propagation pattern allows **Layer 1 components** (providers, tools, memory) to enrich observability spans created in **Layer 2** (core client) without creating tight coupling or breaking architectural boundaries.

### How It Works

```
┌─────────────────────────────────────────────────┐
│ Layer 2: Client (core/client)                  │
│ 1. Creates span                                 │
│ 2. Puts span in context                         │
│ 3. Passes context to Layer 1                    │
└─────────────────────────────────────────────────┘
                    ↓ ctx with span
┌─────────────────────────────────────────────────┐
│ Layer 1: Provider/Tool/Memory                   │
│ 1. Extracts span from context                   │
│ 2. Enriches span with component-specific details│
│ 3. No dependency on observer!                   │
└─────────────────────────────────────────────────┘
```

### Key Principles

1. **Context is the carrier**: Span travels via `context.Context`
2. **Graceful degradation**: If no span in context, operations continue normally
3. **No tight coupling**: Layer 1 doesn't depend on observability package
4. **Standard Go pattern**: Same approach used by OpenTelemetry

### Context Helpers

```go
// Put span in context (Layer 2)
ctx = observability.ContextWithSpan(ctx, span)

// Extract span from context (Layer 1)
span := observability.SpanFromContext(ctx)  // returns nil if not present
```

## Semantic Conventions

Use standardized attribute names for consistency:

### LLM Attributes

```go
observability.AttrLLMProvider      // "llm.provider"
observability.AttrLLMModel          // "llm.model"
observability.AttrLLMEndpoint       // "llm.endpoint"
observability.AttrLLMRequestID      // "llm.request.id"
observability.AttrLLMResponseID     // "llm.response.id"
observability.AttrLLMFinishReason   // "llm.finish_reason"
observability.AttrLLMTemperature    // "llm.temperature"
observability.AttrLLMMaxTokens      // "llm.max_tokens"
```

### Token Attributes

```go
observability.AttrLLMTokensPrompt     // "llm.tokens.prompt"
observability.AttrLLMTokensCompletion // "llm.tokens.completion"
observability.AttrLLMTokensTotal      // "llm.tokens.total"
```

### Tool Attributes

```go
observability.AttrToolName        // "tool.name"
observability.AttrToolDescription // "tool.description"
observability.AttrToolInput       // "tool.input"
observability.AttrToolOutput      // "tool.output"
observability.AttrToolDuration    // "tool.duration"
observability.AttrToolError       // "tool.error"
```

### HTTP Attributes

```go
observability.AttrHTTPMethod          // "http.method"
observability.AttrHTTPStatusCode      // "http.status_code"
observability.AttrHTTPURL             // "http.url"
observability.AttrHTTPRequestBodySize // "http.request.body.size"
observability.AttrHTTPResponseBodySize// "http.response.body.size"
```

### Event Names

```go
observability.EventLLMRequestStart      // "llm.request.start"
observability.EventLLMRequestEnd        // "llm.request.end"
observability.EventToolExecutionStart   // "tool.execution.start"
observability.EventToolExecutionEnd     // "tool.execution.end"
observability.EventTokensReceived       // "llm.tokens.received"
```

### Span Names

```go
observability.SpanClientSendMessage // "client.send_message"
observability.SpanLLMRequest        // "llm.request"
observability.SpanToolExecution     // "tool.execution"
observability.SpanMemoryOperation   // "memory.operation"
```

## Standard Metrics & Spans

### Metrics (Emitted by Client)

**Counters:**
- `aigo.client.request.count` - Total number of requests
  - Attributes: `status` (success/error), `llm.model`
- `aigo.client.tokens.total` - Total tokens used
  - Attributes: `llm.model`
- `aigo.client.tokens.prompt` - Prompt tokens used
  - Attributes: `llm.model`
- `aigo.client.tokens.completion` - Completion tokens used
  - Attributes: `llm.model`

**Histograms:**
- `aigo.client.request.duration` - Request duration in seconds
  - Attributes: `llm.model`

### Spans (Created by Client, Enriched by Providers)

- `client.send_message` - Main request span
  - Created by: Client (Layer 2)
  - Enriched by: LLM Provider, Tools, Memory (Layer 1)
  - Attributes: `llm.model`, `llm.provider`, `llm.tokens.*`, `http.status_code`

## Usage Examples

### Layer 2: Creating and Propagating Span

```go
// In core/client/client.go
func (c *Client[T]) SendMessage(prompt string) (*ai.ChatResponse, error) {
    ctx := context.Background()
    
    if c.observer != nil {
        // Create span
        ctx, span := c.observer.StartSpan(ctx, observability.SpanClientSendMessage,
            observability.String(observability.AttrLLMModel, c.defaultModel),
        )
        defer span.End()
        
        // Put span in context for downstream propagation
        ctx = observability.ContextWithSpan(ctx, span)
    }
    
    // Pass context to provider
    response, err := c.llmProvider.SendMessage(ctx, request)
    // ...
}
```

### Layer 1: Enriching Span in LLM Provider

```go
// In providers/ai/openai/openai.go
func (p *OpenAIProvider) SendMessage(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
    // Extract span from context (nil-safe)
    span := observability.SpanFromContext(ctx)
    
    if span != nil {
        span.AddEvent(observability.EventLLMRequestStart)
        span.SetAttributes(
            observability.String(observability.AttrLLMProvider, "openai"),
            observability.String(observability.AttrLLMEndpoint, p.baseURL),
            observability.String(observability.AttrLLMModel, request.Model),
        )
        defer span.AddEvent(observability.EventLLMRequestEnd)
    }
    
    // Perform API call
    resp, err := p.callAPI(ctx, request)
    
    if span != nil && resp != nil {
        span.SetAttributes(
            observability.String(observability.AttrLLMResponseID, resp.Id),
            observability.String(observability.AttrLLMFinishReason, resp.FinishReason),
            observability.Int(observability.AttrHTTPStatusCode, httpResponse.StatusCode),
        )
        
        if resp.Usage != nil {
            span.AddEvent(observability.EventTokensReceived,
                observability.Int(observability.AttrLLMTokensTotal, resp.Usage.TotalTokens),
            )
        }
    }
    
    return resp, err
}
```

### Layer 1: Enriching Span in Tool

```go
// In providers/tool/tool.go
func (t *Tool[I, O]) Call(ctx context.Context, inputJson string) (string, error) {
    span := observability.SpanFromContext(ctx)
    
    if span != nil {
        span.AddEvent(observability.EventToolExecutionStart,
            observability.String(observability.AttrToolName, t.Name),
            observability.String(observability.AttrToolInput, inputJson),
        )
        defer span.AddEvent(observability.EventToolExecutionEnd)
    }
    
    start := time.Now()
    
    // Parse and execute
    var input I
    if err := json.Unmarshal([]byte(inputJson), &input); err != nil {
        if span != nil {
            span.RecordError(err)
        }
        return "", err
    }
    
    output, err := t.Function(ctx, input)
    duration := time.Since(start)
    
    if span != nil {
        if err != nil {
            span.RecordError(err)
            span.SetAttributes(
                observability.String(observability.AttrToolError, err.Error()),
            )
        } else {
            outputJson, _ := json.Marshal(output)
            span.SetAttributes(
                observability.String(observability.AttrToolOutput, string(outputJson)),
                observability.Duration(observability.AttrToolDuration, duration),
            )
        }
    }
    
    return string(outputJson), nil
}
```

### Layer 1: Enriching Span in Memory Provider

```go
// In providers/memory/inmemory/inmemory.go
func (m *ArrayMemory) AppendMessage(ctx context.Context, message *ai.Message) {
    span := observability.SpanFromContext(ctx)
    
    if span != nil {
        span.AddEvent("memory.append",
            observability.String("message.role", string(message.Role)),
            observability.Int("message.length", len(message.Content)),
        )
    }
    
    m.mu.Lock()
    m.messages = append(m.messages, *message)
    m.mu.Unlock()
    
    if span != nil {
        span.SetAttributes(
            observability.Int("memory.total_messages", len(m.messages)),
        )
    }
}
```

### Example Output

With slog observer at debug level:

```
level=DEBUG msg="Span started" span=client.send_message event=span.start llm.model=gpt-4
level=DEBUG msg="Span event" span=client.send_message event=llm.request.start
level=DEBUG msg="Sending message to LLM" llm.provider=openai llm.endpoint=https://api.openai.com/v1 llm.model=gpt-4
level=DEBUG msg="Span event" span=client.send_message event=tool.execution.start tool.name=calculator
level=DEBUG msg="Span event" span=client.send_message event=tool.execution.end tool.duration=0.002s
level=DEBUG msg="Span event" span=client.send_message event=llm.tokens.received llm.tokens.total=150
level=DEBUG msg="Span event" span=client.send_message event=llm.request.end
level=INFO msg="Message sent successfully" llm.response.id=chatcmpl-123 llm.finish_reason=stop http.status_code=200
level=DEBUG msg="Span ended" span=client.send_message duration=1.234s
```

## Best Practices

### 1. Always Check for Nil Span

```go
span := observability.SpanFromContext(ctx)
if span != nil {
    span.AddEvent("my.event")
}
```

### 2. Use Semantic Conventions

Don't invent attribute names - use the constants:

```go
// ✅ Good
span.SetAttributes(observability.String(observability.AttrLLMProvider, "openai"))

// ❌ Bad
span.SetAttributes(observability.String("provider", "openai"))
```

### 3. Add Events for Significant Moments

```go
span.AddEvent(observability.EventLLMRequestStart)
// ... do work ...
span.AddEvent(observability.EventLLMRequestEnd)
```

### 4. Record Errors

```go
if err != nil {
    if span != nil {
        span.RecordError(err)
        span.SetStatus(observability.StatusError, "Operation failed")
    }
    return err
}
```

### 5. Don't Over-Instrument

Add attributes that are:
- **Valuable** for debugging
- **Low cardinality** (avoid unique IDs in metrics)
- **Consistent** across similar operations

### 6. Pass Context Through Layers

Always accept and pass `context.Context` to enable span propagation:

```go
// ✅ Good
func (p *Provider) DoSomething(ctx context.Context, input string) error

// ❌ Bad
func (p *Provider) DoSomething(input string) error
```

## Performance

Benchmark results (Apple M1 Pro):

| Implementation | StartSpan | Counter | Logging |
|----------------|-----------|---------|---------|
| Nil (default)  | 0 ns     | 0 ns    | 0 ns    |
| Slog           | 124 ns   | 22 ns   | 558 ns  |

**Recommendation**: Use `nil` (default) for production if you don't need observability, slog for development/debugging, and OpenTelemetry (future) for enterprise monitoring.

## Testing

### Mock Observer

```go
type mockObserver struct{}

func (m *mockObserver) StartSpan(ctx context.Context, name string, attrs ...observability.Attribute) (context.Context, observability.Span) {
    return ctx, &mockSpan{}
}

func (m *mockObserver) Counter(name string) observability.Counter {
    return &mockCounter{}
}

// Use in tests
client := client.NewClient[string](provider, client.WithObserver(&mockObserver{}))
```

### Mock Span for Component Testing

```go
type mockSpan struct {
    events []string
    attrs  map[string]interface{}
}

func (m *mockSpan) AddEvent(name string, attrs ...observability.Attribute) {
    m.events = append(m.events, name)
}

func TestProviderEnrichment(t *testing.T) {
    mock := &mockSpan{attrs: make(map[string]interface{})}
    ctx := observability.ContextWithSpan(context.Background(), mock)
    
    provider.SendMessage(ctx, request)
    
    if !contains(mock.events, observability.EventLLMRequestStart) {
        t.Error("Expected LLM request start event")
    }
}
```

## Thread Safety

All implementations are thread-safe:
- **Nil**: No operations performed, inherently safe
- **Slog**: Uses `sync.RWMutex` for metric storage, `slog.Logger` is thread-safe
- **Context propagation**: Safe by design (context is immutable)

## Custom Observer Implementation

To implement a custom observer, implement the `observability.Provider` interface:

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

## Migration Guide

### For Existing Components

1. Add `context.Context` as first parameter if not already present
2. Import observability package
3. Extract span at the beginning: `span := observability.SpanFromContext(ctx)`
4. Add events and attributes at key points
5. Record errors when they occur

No breaking changes required - gracefully degrades if no span present!

## Future Enhancements

- [ ] OpenTelemetry integration with exporters
- [ ] Gauge metrics
- [ ] Distributed tracing with trace/span IDs
- [ ] Sampling strategies
- [ ] Export to Prometheus, Jaeger, DataDog, etc.
- [ ] Automatic instrumentation via code generation
- [ ] Span links for parallel operations
- [ ] Baggage propagation for cross-service tracing

## See Also

- [observability.go](./observability.go) - Core interfaces
- [context.go](./context.go) - Context helpers for span propagation
- [semconv.go](./semconv.go) - Semantic conventions (constants)
- [ARCHITECTURE.md](../../ARCHITECTURE.md) - Overall project architecture
- [examples/observability/main.go](../../examples/observability/main.go) - Complete examples
- [OpenTelemetry Context Propagation](https://opentelemetry.io/docs/instrumentation/go/manual/#context-propagation)
- [Go's log/slog](https://pkg.go.dev/log/slog) documentation