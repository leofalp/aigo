# Slog Observability Provider

A lightweight observability provider implementation using Go's standard library `log/slog`.

## Features

- **Zero external dependencies** - uses only Go's standard library
- **Multiple output formats** - compact, pretty, and JSON formats
- **Configurable log levels** - supports TRACE, DEBUG, INFO, WARN, ERROR via environment variables
- **Environment-based configuration** - automatic setup via `AIGO_LOG_FORMAT` and `AIGO_LOG_LEVEL`
- **Color support** - automatic TTY detection for colored output
- **Metrics tracking** - in-memory counters and histograms
- **Span tracing** - basic distributed tracing support
- **Thread-safe** - safe for concurrent use

## Quick Start

### Using Environment Variables (Recommended)

```go
import (
    "aigo/core/client"
    "aigo/providers/ai/openai"
    "aigo/providers/observability/slogobs"
)

// Uses AIGO_LOG_FORMAT and AIGO_LOG_LEVEL from environment
// Defaults: format=compact, level=INFO
observer := slogobs.New()

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
**Example**:
```
2025-11-03 10:40:35 🔍 TRACE  Using /v1/chat/completions endpoint
                   └─ llm.endpoint: https://api.openai.com/v1
```

To see TRACE logs:

```go
// To see TRACE logs
observer := slogobs.New(
    slogobs.WithFormat(slogobs.FormatPretty),
    slogobs.WithLevel(slog.LevelDebug - 4), // Enable TRACE
)
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
### Development

For local development, use **compact** format with **DEBUG** level:

```bash
export AIGO_LOG_FORMAT=compact
export AIGO_LOG_LEVEL=DEBUG
```

Or for maximum verbosity during debugging, use **pretty** format:

```bash
export AIGO_LOG_FORMAT=pretty
export AIGO_LOG_LEVEL=DEBUG
```

### Production

For production deployments, use **JSON** format with **INFO** level:

```bash
export AIGO_LOG_FORMAT=json
export AIGO_LOG_LEVEL=INFO
```

This provides:
- Structured logs for aggregation tools
- Minimal verbosity
- Easy parsing and filtering
- No ANSI colors

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
### Development - Maximum Verbosity

Show everything including TRACE logs:

```go
observer := slogobs.New(
    slogobs.WithFormat(slogobs.FormatPretty),
    slogobs.WithLevel(slog.LevelDebug - 4), // TRACE level
)
```

Or via environment:
```bash
export AIGO_LOG_FORMAT=pretty
export AIGO_LOG_LEVEL=TRACE
```

### Production - Minimal Output

Show only important events in JSON format:

```go
observer := slogobs.New(
    slogobs.WithFormat(slogobs.FormatJSON),
    slogobs.WithLevel(slog.LevelInfo),
)
```

Or via environment:
```bash
export AIGO_LOG_FORMAT=json
export AIGO_LOG_LEVEL=INFO
```

### Staging - Balanced Approach

Compact format with DEBUG level:

```bash
export AIGO_LOG_FORMAT=compact
export AIGO_LOG_LEVEL=DEBUG
```

## Best Practices

### Choose the Right Format

- **Compact** - Default for development, single-line convenience with structured data
- **Pretty** - Deep debugging sessions, maximum readability
- **JSON** - Production environments, log aggregation tools (ELK, Splunk, etc.)

### Color Support

Colors are automatically enabled for TTY (terminal) output and disabled for pipes/files:

```go
// Auto-detect (recommended)
observer := slogobs.New(slogobs.WithFormat(slogobs.FormatCompact))

// Force colors on/off
observer := slogobs.New(
    slogobs.WithFormat(slogobs.FormatCompact),
    slogobs.WithColors(true),
)
```

### Output Destination

```go
import "os"

// stdout - default, mixed with app output
observer := slogobs.New(slogobs.WithOutput(os.Stdout))

// stderr - separate from app output (recommended for production)
observer := slogobs.New(slogobs.WithOutput(os.Stderr))

// file - write to log file
file, _ := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
observer := slogobs.New(slogobs.WithOutput(file))
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
### Containerized Environments

Use JSON format with stdout - container runtime will capture and ship logs:

```bash
export AIGO_LOG_FORMAT=json
export AIGO_LOG_LEVEL=INFO
```

```go
// Logs written to stdout are captured by Docker/Kubernetes
observer := slogobs.New() // Uses environment variables
```

### CI/CD Pipelines

Use compact format for readable build logs:

```bash
export AIGO_LOG_FORMAT=compact
export AIGO_LOG_LEVEL=INFO
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