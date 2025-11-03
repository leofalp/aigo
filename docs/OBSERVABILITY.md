# Observability in AIGO

AIGO provides a comprehensive, yet lightweight observability system that supports tracing, metrics, and structured logging. The observability system is designed to be:

- **Optional**: Zero overhead when not enabled (nil observer)
- **Provider-agnostic**: Works with any implementation (slog, OpenTelemetry, custom)
- **Context-aware**: Propagates through the call stack via Go contexts
- **OpenTelemetry-compatible**: All semantic conventions map to OTel standards

## Table of Contents

- [Quick Start](#quick-start)
- [Logging Levels](#logging-levels)
- [Semantic Conventions](#semantic-conventions)
- [OpenTelemetry Mapping](#opentelemetry-mapping)
- [Built-in Providers](#built-in-providers)
- [Custom Providers](#custom-providers)
- [Best Practices](#best-practices)

## Quick Start

### No Observability (Default)

By default, clients have no observability overhead:

```go
client, err := client.NewClient[string](
    openai.NewOpenAIProvider(),
)
// No observability - zero overhead
```

### With Slog Observability

```go
import (
    "aigo/providers/observability/slogobs"
	"log/slog"
)

logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

client, err := client.NewClient[string](
    openai.NewOpenAIProvider(),
    client.WithObserver(slogobs.New(logger)),
)
```

## Logging Levels

AIGO uses four distinct logging levels, each serving a specific purpose:

### TRACE (Most Verbose)

**Purpose**: Low-level protocol and transport details

**Use Cases**:
- HTTP request/response inspection
- Raw payload sizes
- Network timing breakdown
- Protocol-level debugging

**Example Output**:
```
TRACE: OpenAI provider preparing request
  llm.provider=openai
  llm.endpoint=https://api.openai.com/v1
  request.messages_count=3
  request.tools_count=2

TRACE: Using /v1/chat/completions endpoint
  llm.endpoint=https://api.openai.com/v1/chat/completions
  use_legacy_functions=false
```

**When to Enable**: Debugging network issues, analyzing API calls, investigating timeouts

---

### DEBUG (Detailed Technical)

**Purpose**: Technical details for debugging application logic

**Use Cases**:
- Span lifecycle events (start/end)
- Memory operations (append, clear)
- Token usage breakdowns
- Metrics recording
- Message content (truncated)

**Example Output**:
```
DEBUG: Span started
  span=client.send_message
  event=span.start
  llm.model=gpt-4

DEBUG: Message content
  client.prompt="What is the capital of France?"

DEBUG: Using stateful mode with memory
  memory.total_messages=5

DEBUG: Token usage
  llm.tokens.total=144
  llm.tokens.prompt=25
  llm.tokens.completion=119

DEBUG: Response content
  response.content="The capital of France is Paris."
```

**When to Enable**: Application debugging, understanding flow, investigating token usage

---

### INFO (Operations Summary)

**Purpose**: High-level operational events and summaries

**Use Cases**:
- Operation start/completion
- Success/failure status
- Key metrics (duration, model, finish reason)
- Span summaries with context

**Example Output**:
```
INFO: Sending message to LLM
  llm.model=gpt-4
  client.tools_count=2

INFO: Message sent successfully
  llm.model=gpt-4
  llm.finish_reason=stop
  duration=1.234s
  client.tool_calls=0

INFO: Span ended
  span=client.send_message
  duration=1.234s
  llm.tokens.total=144
  status=ok
```

**When to Enable**: Production monitoring, understanding system behavior, audit trails

---

### WARN/ERROR (Problems)

**Purpose**: Issues, failures, and exceptional conditions

**Use Cases**:
- API errors
- Validation failures
- Timeouts
- Retry attempts

**Example Output**:
```
ERROR: Failed to send message to LLM
  error="API key is not set"
  duration=0.001s
  llm.model=gpt-4
```

**When to Enable**: Always (production error tracking)

---

## Semantic Conventions

All attribute names follow standardized semantic conventions defined in `providers/observability/semconv.go`. This ensures consistency across components and compatibility with OpenTelemetry.

### Key Attribute Categories

#### LLM Provider Attributes
- `llm.provider` - Provider name (e.g., "openai", "anthropic")
- `llm.model` - Model identifier (e.g., "gpt-4", "claude-3")
- `llm.endpoint` - API endpoint URL
- `llm.request.id` - Unique request identifier
- `llm.response.id` - Unique response identifier
- `llm.finish_reason` - Why generation stopped

#### Token Usage
- `llm.tokens.total` - Total tokens used
- `llm.tokens.prompt` - Prompt/input tokens
- `llm.tokens.completion` - Completion/output tokens

#### HTTP Attributes
- `http.method` - HTTP method (GET, POST, etc.)
- `http.status_code` - Response status code
- `http.url` - Full request URL
- `http.request.body.size` - Request payload size in bytes
- `http.response.body.size` - Response payload size in bytes

#### Memory Attributes
- `memory.message.role` - Message role (user, assistant, system)
- `memory.message.length` - Message content length
- `memory.total_messages` - Total messages in memory

#### Client Attributes
- `client.prompt` - User input (truncated if long)
- `client.tools_count` - Number of available tools
- `client.tool_calls` - Number of tool calls in response

### Span Names

- `client.send_message` - Client-level message send operation
- `llm.request` - LLM API request
- `tool.execution` - Tool execution
- `memory.operation` - Memory operation

### Event Names

- `llm.request.start` - LLM request starting
- `llm.request.end` - LLM request completed
- `llm.tokens.received` - Tokens received from LLM
- `memory.append` - Message appended to memory
- `http.request.prepared` - HTTP request prepared
- `http.response.received` - HTTP response received

### Metric Names

- `aigo.client.request.count` - Counter: Total requests
- `aigo.client.request.duration` - Histogram: Request duration in seconds
- `aigo.client.tokens.total` - Counter: Total tokens used
- `aigo.client.tokens.prompt` - Counter: Prompt tokens used
- `aigo.client.tokens.completion` - Counter: Completion tokens used

## OpenTelemetry Mapping

AIGO's observability system is designed to map directly to OpenTelemetry (OTel) primitives:

### Spans → OTel Spans

| AIGO Concept | OTel Equivalent | Notes |
|--------------|-----------------|-------|
| `Span` interface | `trace.Span` | Direct mapping |
| `StartSpan()` | `tracer.Start()` | Creates new span |
| `Span.End()` | `span.End()` | Completes span |
| `SetAttributes()` | `span.SetAttributes()` | Adds metadata |
| `SetStatus()` | `span.SetStatus()` | Sets span status |
| `RecordError()` | `span.RecordError()` | Records error event |
| `AddEvent()` | `span.AddEvent()` | Adds span event |

### Status Codes → OTel Status

| AIGO Status | OTel Status | Description |
|-------------|-------------|-------------|
| `StatusUnset` | `codes.Unset` | No status set |
| `StatusOK` | `codes.Ok` | Success |
| `StatusError` | `codes.Error` | Failure |

### Metrics → OTel Metrics

| AIGO Metric | OTel Equivalent | Unit |
|-------------|-----------------|------|
| `Counter` | `Int64Counter` | count |
| `Histogram` | `Float64Histogram` | seconds/bytes |

### Attributes → OTel Attributes

All AIGO semantic conventions follow OTel naming patterns:
- Dot-separated namespaces (e.g., `llm.model`, `http.status_code`)
- Lowercase with underscores
- Consistent with [OTel Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)

### Context Propagation

AIGO uses Go's `context.Context` for propagation, compatible with OTel's context propagation:

```go
// AIGO
ctx = observability.ContextWithSpan(ctx, span)
ctx = observability.ContextWithObserver(ctx, observer)

// OpenTelemetry (compatible approach)
ctx = trace.ContextWithSpan(ctx, span)
```

## Built-in Providers

### Slog Provider

Uses Go's standard library `log/slog`:

```go
import (
    "aigo/providers/observability/slogobs"
    "log/slog"
)

// Text format
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

// JSON format
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

observer := slogobs.New(logger)
```

**Features**:
- Zero external dependencies
- Thread-safe
- Built-in level filtering
- Structured output (text or JSON)

## Custom Providers

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

### Example: OpenTelemetry Provider

```go
type OtelProvider struct {
    tracer trace.Tracer
    meter  metric.Meter
    logger *slog.Logger
}

func (p *OtelProvider) StartSpan(ctx context.Context, name string, attrs ...observability.Attribute) (context.Context, observability.Span) {
    otelAttrs := make([]attribute.KeyValue, len(attrs))
    for i, attr := range attrs {
        otelAttrs[i] = attribute.String(attr.Key, fmt.Sprint(attr.Value))
    }
    
    ctx, span := p.tracer.Start(ctx, name, trace.WithAttributes(otelAttrs...))
    return ctx, &OtelSpan{span: span}
}

// ... implement other methods
```

## Best Practices

### 1. Use Semantic Conventions

Always use constants from `observability/semconv.go`:

```go
// Good ✓
observer.Info(ctx, "Message sent",
    observability.String(observability.AttrLLMModel, model),
)

// Bad ✗
observer.Info(ctx, "Message sent",
    observability.String("model", model),
)
```

### 2. Truncate Long Strings

Use `TruncateString()` for potentially long content:

```go
truncated := observability.TruncateStringDefault(longMessage)
observer.Debug(ctx, "Message content",
    observability.String(observability.AttrClientPrompt, truncated),
)
```

### 3. Appropriate Logging Levels

- **TRACE**: Protocol/transport details only
- **DEBUG**: Technical debugging information
- **INFO**: Business operations and summaries
- **WARN/ERROR**: Problems only

### 4. Context Propagation

Always propagate context with observer and span:

```go
if observer != nil {
    ctx, span := observer.StartSpan(ctx, "operation")
    defer span.End()
    
    ctx = observability.ContextWithSpan(ctx, span)
    ctx = observability.ContextWithObserver(ctx, observer)
    
    // Pass ctx to downstream calls
    someFunction(ctx, ...)
}
```

### 5. Nil-Safe Operations

Always check if observer is nil before using:

```go
if c.observer != nil {
    c.observer.Info(ctx, "Operation completed")
}
```

### 6. Metrics Best Practices

- Use counters for cumulative values (requests, tokens)
- Use histograms for distributions (duration, sizes)
- Add dimensions via attributes for filtering/grouping

```go
observer.Counter(observability.MetricClientRequestCount).Add(ctx, 1,
    observability.String(observability.AttrStatus, "success"),
    observability.String(observability.AttrLLMModel, model),
)
```

### 7. Span Status

Always set span status to indicate success/failure:

```go
if err != nil {
    span.RecordError(err)
    span.SetStatus(observability.StatusError, "Failed to process")
} else {
    span.SetStatus(observability.StatusOK, "Successfully processed")
}
```

## Examples

See the following examples in the repository:

- `examples/layer2/observability/` - Comprehensive observability demonstration
- `core/client/client_observability_test.go` - Unit tests with mock observer
- `providers/observability/slog/` - Reference implementation

## Future Enhancements

Planned improvements:

1. **OpenTelemetry Provider**: Native OTel implementation
2. **Sampling**: Configurable trace sampling rates
3. **Baggage**: Cross-service context propagation
4. **Exporters**: Jaeger, Prometheus, Zipkin support
5. **Auto-instrumentation**: Automatic span creation for common operations

## Contributing

When adding new observability points:

1. Add semantic conventions to `semconv.go`
2. Use appropriate logging levels
3. Include relevant attributes
4. Document in this file
5. Add tests
6. Update examples if needed