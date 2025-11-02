# Span Propagation Pattern

This document explains how spans are propagated through the application layers using Go's `context.Context`, enabling fine-grained observability at every level.

## Overview

The span propagation pattern allows **Layer 1 components** (providers, tools) to enrich observability spans created in **Layer 2** (core client) without creating tight coupling or breaking architectural boundaries.

## How It Works

```
┌─────────────────────────────────────────────────┐
│ Layer 2: Client (core/client)                  │
│ 1. Creates span                                 │
│ 2. Puts span in context                         │
│ 3. Passes context to Layer 1                    │
└─────────────────────────────────────────────────┘
                    ↓ ctx with span
┌─────────────────────────────────────────────────┐
│ Layer 1: Provider (providers/ai/openai)        │
│ 1. Extracts span from context                   │
│ 2. Enriches span with provider-specific details │
│ 3. No dependency on observer!                   │
└─────────────────────────────────────────────────┘
```

## Key Principles

1. **Context is the carrier**: Span travels via `context.Context`
2. **Graceful degradation**: If no span in context, operations continue normally
3. **No tight coupling**: Layer 1 doesn't depend on observability package
4. **Standard Go pattern**: Same approach used by OpenTelemetry

## API Reference

### Context Helpers

```go
// Put span in context
ctx = observability.ContextWithSpan(ctx, span)

// Extract span from context (returns nil if not present)
span := observability.SpanFromContext(ctx)
```

### Semantic Conventions

Use standardized attribute names for consistency:

```go
// LLM attributes
observability.AttrLLMProvider      // "llm.provider"
observability.AttrLLMModel          // "llm.model"
observability.AttrLLMEndpoint       // "llm.endpoint"
observability.AttrLLMResponseID     // "llm.response.id"
observability.AttrLLMFinishReason   // "llm.finish_reason"

// Token attributes
observability.AttrLLMTokensTotal      // "llm.tokens.total"
observability.AttrLLMTokensPrompt     // "llm.tokens.prompt"
observability.AttrLLMTokensCompletion // "llm.tokens.completion"

// Tool attributes
observability.AttrToolName        // "tool.name"
observability.AttrToolInput       // "tool.input"
observability.AttrToolOutput      // "tool.output"
observability.AttrToolDuration    // "tool.duration"

// HTTP attributes
observability.AttrHTTPStatusCode  // "http.status_code"
observability.AttrHTTPMethod      // "http.method"

// Event names
observability.EventLLMRequestStart    // "llm.request.start"
observability.EventLLMRequestEnd      // "llm.request.end"
observability.EventToolExecutionStart // "tool.execution.start"
observability.EventToolExecutionEnd   // "tool.execution.end"
```

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

### Layer 1: Enriching Span in Provider

```go
// In providers/ai/openai/openai.go
func (p *OpenAIProvider) SendMessage(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
    // Extract span from context (nil-safe)
    span := observability.SpanFromContext(ctx)
    
    if span != nil {
        // Add provider-specific details
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
        // Enrich with response details
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
    
    // Parse input
    var input I
    if err := json.Unmarshal([]byte(inputJson), &input); err != nil {
        if span != nil {
            span.RecordError(err)
        }
        return "", err
    }
    
    // Execute tool
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
    
    // ... rest of the logic
}
```

## Benefits

### ✅ Architectural Cleanliness
- Layer 1 doesn't depend on Layer 2 or observability implementation
- Maintains independence of layers
- Can be used or ignored without breaking anything

### ✅ Fine-Grained Visibility
- See exactly what happens inside provider API calls
- Track token usage at the source
- Monitor tool execution performance
- Correlate HTTP status codes with outcomes

### ✅ Flexibility
- Works with any observability backend (slog, OpenTelemetry, etc.)
- Each component can add its own relevant details
- Easy to extend with custom attributes

### ✅ Standards Compliance
- Follows OpenTelemetry semantic conventions pattern
- Compatible with distributed tracing systems
- Easy migration path to full OpenTelemetry

## Example Output

With slog observer at debug level, you'll see:

```
level=DEBUG msg="Span started" span=client.send_message event=span.start model=gpt-4
level=DEBUG msg="Span event" span=client.send_message event=llm.request.start
level=DEBUG msg="Sending message to LLM" llm.provider=openai llm.endpoint=https://api.openai.com/v1 llm.model=gpt-4
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
- **Low cardinality** (avoid IDs in metrics)
- **Consistent** across similar operations

## Testing

Mock the span for tests:

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
    
    // Verify enrichment
    if !contains(mock.events, observability.EventLLMRequestStart) {
        t.Error("Expected LLM request start event")
    }
}
```

## Migration Guide

### For Existing Providers

1. Import observability package
2. Extract span at the beginning of main method
3. Add events and attributes at key points
4. No changes to function signatures needed!

### For New Providers

Follow the pattern in `providers/ai/openai/openai.go` as reference implementation.

## Future Enhancements

- [ ] Automatic span creation for nested operations
- [ ] Span links for parallel operations
- [ ] Baggage propagation for cross-service tracing
- [ ] Integration with OpenTelemetry exporters
- [ ] Automatic instrumentation via code generation

## See Also

- [observability.go](./observability.go) - Core interfaces
- [context.go](./context.go) - Context helpers
- [semconv.go](./semconv.go) - Semantic conventions
- [OpenTelemetry Context Propagation](https://opentelemetry.io/docs/instrumentation/go/manual/#context-propagation)