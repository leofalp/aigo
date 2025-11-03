# Log Output Formats

This document shows examples of the three log output formats available in slogobs.

## Format Comparison

### Compact Format (Default)

**Best for**: Development, single-line convenience with structured data

**Characteristics**:
- Single line per log entry
- Attributes in JSON format
- Clear separator (`→`) between message and attributes
- Supports colors in terminal

**Example**:
```
2025-11-03 10:40:35 TRACE OpenAI provider preparing request → {"llm.provider":"openai","llm.endpoint":"https://api.openai.com/v1","llm.model":"gpt-4","request.messages_count":1}
2025-11-03 10:40:35 DEBUG Span started → {"span":"client.send_message","event":"span.start"}
2025-11-03 10:40:35  INFO Sending message to LLM → {"llm.model":"gpt-4","client.tools_count":0}
2025-11-03 10:40:35  WARN Rate limit approaching → {"current":95,"limit":100}
2025-11-03 10:40:35 ERROR Failed to send message → {"error":"connection timeout","duration":5000}
```

**Configuration**:
```go
observer := slogobs.New(
    slogobs.WithFormat(slogobs.FormatCompact),
    slogobs.WithLevel(slog.LevelDebug),
)
```

Or via environment:
```bash
export AIGO_LOG_FORMAT=compact
export AIGO_LOG_LEVEL=DEBUG
```

---

### Pretty Format

**Best for**: Deep debugging sessions, maximum readability

**Characteristics**:
- Multi-line format with emoji icons
- Tree-style indented attributes
- Each attribute on its own line
- Supports colors in terminal

**Example**:
```
2025-11-03 10:40:35 🔵 DEBUG  Span started
                   ├─ span: client.send_message
                   └─ event: span.start

2025-11-03 10:40:35 🟢 INFO   Sending message to LLM
                   ├─ llm.model: gpt-4
                   └─ client.tools_count: 0

2025-11-03 10:40:35 🔵 DEBUG  Using stateful mode with memory
                   └─ memory.total_messages: 1

2025-11-03 10:40:35 🔍 TRACE  OpenAI provider preparing request
                   ├─ llm.provider: openai
                   ├─ llm.endpoint: https://api.openai.com/v1
                   ├─ llm.model: gpt-4
                   └─ request.messages_count: 1

2025-11-03 10:40:35 🟡 WARN   Rate limit approaching
                   ├─ current: 95
                   └─ limit: 100

2025-11-03 10:40:35 🔴 ERROR  Failed to send message
                   ├─ error: connection timeout
                   └─ duration: 5000
```

**Configuration**:
```go
observer := slogobs.New(
    slogobs.WithFormat(slogobs.FormatPretty),
    slogobs.WithLevel(slog.LevelDebug),
)
```

Or via environment:
```bash
export AIGO_LOG_FORMAT=pretty
export AIGO_LOG_LEVEL=DEBUG
```

---

### JSON Format

**Best for**: Production environments, log aggregation tools (ELK, Splunk, Datadog, etc.)

**Characteristics**:
- One JSON object per line
- Standard fields: `time`, `level`, `msg`
- All attributes merged at root level
- Machine-readable, easy to parse and query
- No colors

**Example**:
```json
{"time":"2025-11-03T10:40:35","level":"DEBUG","msg":"Span started","span":"client.send_message","event":"span.start"}
{"time":"2025-11-03T10:40:35","level":"INFO","msg":"Sending message to LLM","llm.model":"gpt-4","client.tools_count":0}
{"time":"2025-11-03T10:40:35","level":"DEBUG","msg":"Using stateful mode with memory","memory.total_messages":1}
{"time":"2025-11-03T10:40:35","level":"TRACE","msg":"OpenAI provider preparing request","llm.provider":"openai","llm.endpoint":"https://api.openai.com/v1","llm.model":"gpt-4","request.messages_count":1}
{"time":"2025-11-03T10:40:35","level":"WARN","msg":"Rate limit approaching","current":95,"limit":100}
{"time":"2025-11-03T10:40:35","level":"ERROR","msg":"Failed to send message","error":"connection timeout","duration":5000}
```

**Configuration**:
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

---

## Choosing the Right Format

### Development Workflow

```bash
# Local development - compact format for quick scanning
export AIGO_LOG_FORMAT=compact
export AIGO_LOG_LEVEL=DEBUG

# Investigating a bug - pretty format for detailed analysis
export AIGO_LOG_FORMAT=pretty
export AIGO_LOG_LEVEL=TRACE

# Testing integrations - compact format with INFO level
export AIGO_LOG_FORMAT=compact
export AIGO_LOG_LEVEL=INFO
```

### Production Deployments

```bash
# Standard production setup
export AIGO_LOG_FORMAT=json
export AIGO_LOG_LEVEL=INFO

# High-traffic production (reduce verbosity)
export AIGO_LOG_FORMAT=json
export AIGO_LOG_LEVEL=WARN

# Troubleshooting production issues (temporary)
export AIGO_LOG_FORMAT=json
export AIGO_LOG_LEVEL=DEBUG
```

### CI/CD Pipelines

```bash
# Unit tests - minimal output
export AIGO_LOG_FORMAT=compact
export AIGO_LOG_LEVEL=WARN

# Integration tests - detailed output
export AIGO_LOG_FORMAT=compact
export AIGO_LOG_LEVEL=DEBUG

# Build logs - structured for parsing
export AIGO_LOG_FORMAT=json
export AIGO_LOG_LEVEL=INFO
```

---

## Log Level Hierarchy

From most to least verbose:

1. **TRACE** (`slog.LevelDebug - 4`) - Very detailed, internal operations
2. **DEBUG** (`slog.LevelDebug`) - Diagnostic information
3. **INFO** (`slog.LevelInfo`) - Informational messages (default)
4. **WARN** (`slog.LevelWarn`) - Warning conditions
5. **ERROR** (`slog.LevelError`) - Error conditions

When you set a level, all messages at that level and above are logged.

Example:
- `AIGO_LOG_LEVEL=INFO` → Shows INFO, WARN, ERROR
- `AIGO_LOG_LEVEL=DEBUG` → Shows DEBUG, INFO, WARN, ERROR
- `AIGO_LOG_LEVEL=TRACE` → Shows everything

---

## Color Support

Colors are automatically enabled when output is a terminal (TTY) and disabled for pipes/files.

**Compact and Pretty formats** support colors:
- 🔴 **ERROR** - Red
- 🟡 **WARN** - Yellow
- 🟢 **INFO** - Green
- 🔵 **DEBUG** - Blue
- ⚪ **TRACE** - Gray

**JSON format** never uses colors (for parsing compatibility).

Force color on/off:
```go
observer := slogobs.New(
    slogobs.WithFormat(slogobs.FormatCompact),
    slogobs.WithColors(true), // Force colors on
)
```

---

## Examples by Use Case

### Debugging a Single Request

```bash
# Use pretty format to see all details
export AIGO_LOG_FORMAT=pretty
export AIGO_LOG_LEVEL=TRACE
go run main.go
```

### Running Tests

```bash
# Use compact format for readable output
export AIGO_LOG_FORMAT=compact
export AIGO_LOG_LEVEL=DEBUG
go test ./...
```

### Production Monitoring

```bash
# Use JSON format for log aggregation
export AIGO_LOG_FORMAT=json
export AIGO_LOG_LEVEL=INFO
./app | tee -a /var/log/app.log
```

### Docker/Kubernetes

```yaml
# docker-compose.yml
services:
  app:
    environment:
      - AIGO_LOG_FORMAT=json
      - AIGO_LOG_LEVEL=INFO
```

```yaml
# kubernetes deployment
env:
  - name: AIGO_LOG_FORMAT
    value: "json"
  - name: AIGO_LOG_LEVEL
    value: "INFO"
```
