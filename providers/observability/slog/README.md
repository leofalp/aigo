# Slog Observability Provider

A lightweight observability provider implementation using Go's standard library `log/slog`.

## Features

- **Zero External Dependencies**: Uses only Go standard library
- **Thread-Safe**: Safe for concurrent use
- **Structured Logging**: Key-value pairs for easy parsing
- **Multiple Formats**: Text or JSON output
- **Level Filtering**: TRACE, DEBUG, INFO, WARN, ERROR
- **In-Memory Metrics**: Thread-safe counters and histograms

## Quick Start

```go
import (
    "aigo/core/client"
    "aigo/providers/ai/openai"
    "aigo/providers/observability/slog"
    logslog "log/slog"
    "os"
)

// Create logger with desired level and format
logger := logslog.New(logslog.NewTextHandler(os.Stdout, &logslog.HandlerOptions{
    Level: logslog.LevelInfo,
}))

// Create observer
observer := slog.New(logger)

// Use with client
client, err := client.NewClient[string](
    openai.NewOpenAIProvider(),
    client.WithObserver(observer),
)
```

## Logging Levels

### TRACE (LevelDebug-4)

**Most verbose level** - Protocol and transport details.

Use for:
- HTTP request/response inspection
- Raw payload debugging
- Network timing analysis
- Low-level protocol debugging

Example output:
```
time=2025-11-02T17:28:57.310+01:00 level=DEBUG-4 msg="OpenAI provider preparing request" llm.provider=openai llm.endpoint=https://api.openai.com/v1 request.messages_count=3
time=2025-11-02T17:28:57.310+01:00 level=DEBUG-4 msg="Using /v1/chat/completions endpoint" llm.endpoint=https://api.openai.com/v1/chat/completions
```

**Note**: TRACE uses `slog.LevelDebug-4`, which means you need to set your handler level to a lower value to see these logs:

```go
// To see TRACE logs
logger := logslog.New(logslog.NewTextHandler(os.Stdout, &logslog.HandlerOptions{
    Level: logslog.LevelDebug - 4, // Enable TRACE
}))
```

### DEBUG

**Detailed technical information** - For debugging application logic.

Use for:
- Span lifecycle events
- Memory operations
- Token usage details
- Metrics recording
- Message content (truncated)

Example output:
```
time=2025-11-02T17:28:57.310+01:00 level=DEBUG msg="Span started" span=client.send_message event=span.start llm.model=gpt-4
time=2025-11-02T17:28:57.310+01:00 level=DEBUG msg="Message content" client.prompt="What is the capital of France?"
time=2025-11-02T17:28:57.310+01:00 level=DEBUG msg="Token usage" llm.tokens.total=144 llm.tokens.prompt=25 llm.tokens.completion=119
```

### INFO

**Operational summaries** - High-level events and outcomes.

Use for:
- Operation start/completion
- Success status
- Key metrics summary
- Production monitoring

Example output:
```
time=2025-11-02T17:28:57.310+01:00 level=INFO msg="Sending message to LLM" llm.model=gpt-4 client.tools_count=2
time=2025-11-02T17:29:00.166+01:00 level=INFO msg="Message sent successfully" llm.model=gpt-4 llm.finish_reason=stop duration=2.856s client.tool_calls=0
time=2025-11-02T17:29:00.166+01:00 level=INFO msg="Span ended" span=client.send_message duration=2.856s status=ok
```

### WARN / ERROR

**Problems and failures** - Issues requiring attention.

Use for:
- API errors
- Validation failures
- Timeouts
- Recoverable errors

Example output:
```
time=2025-11-02T17:28:57.310+01:00 level=ERROR msg="Failed to send message to LLM" error="API key is not set" duration=0.001s llm.model=gpt-4
```

## Output Formats

### Text Format

Human-readable format, great for development:

```go
logger := logslog.New(logslog.NewTextHandler(os.Stdout, &logslog.HandlerOptions{
    Level: logslog.LevelInfo,
}))
```

Output:
```
time=2025-11-02T17:28:57.310+01:00 level=INFO msg="Message sent" model=gpt-4 duration=1.234s
```

### JSON Format

Machine-readable format, ideal for production and log aggregation:

```go
logger := logslog.New(logslog.NewJSONHandler(os.Stdout, &logslog.HandlerOptions{
    Level: logslog.LevelInfo,
}))
```

Output:
```json
{"time":"2025-11-02T17:28:57.310+01:00","level":"INFO","msg":"Message sent","model":"gpt-4","duration":1.234}
```

## Metrics

The slog provider tracks metrics in memory with thread-safe counters and histograms.

### Counters

Monotonically increasing values:

```go
counter := observer.Counter("aigo.client.request.count")
counter.Add(ctx, 1,
    observability.String("status", "success"),
    observability.String("model", "gpt-4"),
)
```

Output:
```
level=DEBUG msg=Counter metric=aigo.client.request.count type=counter value=1 delta=1 status=success model=gpt-4
```

### Histograms

Distribution of values (duration, sizes):

```go
histogram := observer.Histogram("aigo.client.request.duration")
histogram.Record(ctx, 1.234,
    observability.String("model", "gpt-4"),
)
```

Output:
```
level=DEBUG msg=Histogram metric=aigo.client.request.duration type=histogram value=1.234 model=gpt-4
```

**Note**: Metrics are logged at DEBUG level. The in-memory store maintains cumulative values for counters.

## Spans

Spans represent units of work and follow the lifecycle: start → events → end.

### Span Start

```
level=DEBUG msg="Span started" span=client.send_message event=span.start llm.model=gpt-4 client.prompt="..."
```

### Span Events

Events are logged during span execution:

```
level=DEBUG msg="Span event" span=client.send_message event=llm.request.start
level=DEBUG msg="Span event" span=client.send_message event=http.request.prepared http.method=POST http.url=...
level=DEBUG msg="Span event" span=client.send_message event=http.response.received http.status_code=200
level=DEBUG msg="Span event" span=client.send_message event=llm.request.end
```

### Span End

Span end is logged at **INFO** level with all accumulated attributes:

```
level=INFO msg="Span ended" span=client.send_message duration=2.856s llm.model=gpt-4 status=ok status_description="Message sent successfully"
```

## Thread Safety

All operations are thread-safe:
- Spans use `sync.Mutex` for attribute updates
- Metrics store uses `sync.RWMutex` for concurrent access
- Safe to use from multiple goroutines

## Level Configuration

### Development

Show everything for debugging:

```go
logger := logslog.New(logslog.NewTextHandler(os.Stdout, &logslog.HandlerOptions{
    Level: logslog.LevelDebug - 4, // Shows TRACE, DEBUG, INFO, WARN, ERROR
}))
```

### Production

Show only important events:

```go
logger := logslog.New(logslog.NewJSONHandler(os.Stdout, &logslog.HandlerOptions{
    Level: logslog.LevelInfo, // Shows INFO, WARN, ERROR only
}))
```

### Troubleshooting

Enable DEBUG for more details:

```go
logger := logslog.New(logslog.NewJSONHandler(os.Stdout, &logslog.HandlerOptions{
    Level: logslog.LevelDebug, // Shows DEBUG, INFO, WARN, ERROR
}))
```

## Best Practices

### 1. Use JSON in Production

JSON format is easier to parse, index, and query:

```go
// Production
logger := logslog.New(logslog.NewJSONHandler(os.Stdout, &logslog.HandlerOptions{
    Level: logslog.LevelInfo,
}))
```

### 2. Text for Development

Text format is more readable during development:

```go
// Development
logger := logslog.New(logslog.NewTextHandler(os.Stdout, &logslog.HandlerOptions{
    Level: logslog.LevelDebug,
}))
```

### 3. Level Selection

- **Production**: `LevelInfo` (or `LevelWarn` for high-volume systems)
- **Staging**: `LevelDebug`
- **Development**: `LevelDebug` or `LevelDebug-4` (TRACE)
- **Troubleshooting**: Temporarily set to `LevelDebug` or `LevelDebug-4`

### 4. Output Destination

Write to stderr in production to separate from application output:

```go
logger := logslog.New(logslog.NewJSONHandler(os.Stderr, &logslog.HandlerOptions{
    Level: logslog.LevelInfo,
}))
```

### 5. Structured Fields

Always use structured attributes instead of formatting messages:

```go
// Good ✓
observer.Info(ctx, "Message sent",
    observability.String("model", "gpt-4"),
    observability.Int("tokens", 144),
)

// Bad ✗
observer.Info(ctx, fmt.Sprintf("Message sent with model %s and %d tokens", "gpt-4", 144))
```

## Integration with Log Aggregation

The slog provider works well with log aggregation systems:

### Elasticsearch / OpenSearch

Use JSON handler and ship logs via Filebeat or Fluentd:

```go
file, _ := os.OpenFile("/var/log/aigo.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
logger := logslog.New(logslog.NewJSONHandler(file, &logslog.HandlerOptions{
    Level: logslog.LevelInfo,
}))
```

### CloudWatch / Datadog

Use JSON handler with appropriate log shipping:

```go
logger := logslog.New(logslog.NewJSONHandler(os.Stdout, &logslog.HandlerOptions{
    Level: logslog.LevelInfo,
}))
// Logs are captured by container runtime and shipped
```

## Limitations

1. **In-Memory Metrics Only**: Metrics are not persisted or exported
2. **No Distributed Tracing**: No automatic trace context propagation across services
3. **No Sampling**: All logs are emitted (filtered by level only)

For advanced use cases, consider implementing an OpenTelemetry provider.

## Examples

See `examples/layer2/observability/main.go` for complete examples with all logging levels.

## Performance

- **Zero Overhead**: When observer is nil, no performance impact
- **Minimal Overhead**: With observer, only synchronization overhead for metrics
- **Thread-Safe**: Safe for concurrent use across goroutines
- **No External Dependencies**: Only Go standard library

## See Also

- [Observability Documentation](../../../docs/OBSERVABILITY.md)
- [Semantic Conventions](../semconv.go)
- [Example](../../../examples/layer2/observability/)