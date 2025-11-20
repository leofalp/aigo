# Type-Safe ReAct Example

This example demonstrates the **type-safe ReAct (Reasoning + Acting)** pattern in `aigo`, which combines automatic tool execution loops with structured output parsing.

## What is ReAct?

ReAct is a pattern where an LLM:
1. **Reasons** about what it needs to do
2. **Acts** by calling tools to gather information
3. **Repeats** until it has enough information
4. **Returns** a final answer (optionally in a structured format)

## Running the Example

```bash
cd examples/layer3/react
go run main.go
```

**Requirements**:
- OpenAI API key (set in `.env` or environment variable)
- Internet connection (for tool execution)

## What's Included

This example shows **three different usage patterns** in one file:

### Example 1: Type-Safe Math Agent

Demonstrates structured output with calculator tool.

```go
type MathResult struct {
    Answer      int    `json:"answer" jsonschema:"required"`
    Explanation string `json:"explanation" jsonschema:"required"`
    Confidence  string `json:"confidence" jsonschema:"required"`
}

agent := react.New[MathResult](baseClient)
result, _ := agent.Execute(ctx, "What is the sum of the first 5 prime numbers?")

// Type-safe access!
answer := result.Data.Answer
```

**Key Points**:
- Define output structure with Go types
- Automatic JSON schema generation (injected at construction)
- Type-safe field access
- No manual parsing needed
- No extra LLM call for structured output

### Example 2: Type-Safe Research Agent

Demonstrates structured output with search tool and complex types.

```go
type ResearchResult struct {
    Topic       string   `json:"topic" jsonschema:"required"`
    Summary     string   `json:"summary" jsonschema:"required"`
    KeyPoints   []string `json:"key_points" jsonschema:"required"`
    Sources     int      `json:"sources" jsonschema:"required"`
    Reliability string   `json:"reliability" jsonschema:"required"`
}

agent := react.New[ResearchResult](baseClient)
result, _ := agent.Execute(ctx, "Research quantum computing developments")

// Type-safe array access!
for _, point := range result.Data.KeyPoints {
    fmt.Println(point)
}
```

**Key Points**:
- Works with arrays and complex structures
- Different type than Example 1 (shows flexibility)
- Automatic schema constraint enforcement
- Smart retry on parse failure

### Example 3: Untyped (Backward Compatible)

Shows untyped usage for when you don't need structured output.

```go
// Use string for untyped text results (default)
agent := react.New[string](baseClient)

result, _ := agent.Execute(ctx, prompt)

// Access as string
text := *result.Data
```

**Key Points**:
- Default untyped type is `string`, not `interface{}`
- Backward compatible with old API
- Use when output structure varies
- Good for prototyping

## Key Features

### 🎯 Type Safety
- Define expected output with Go structs
- Compile-time type checking
- No casting, no reflection in user code
- Schema injected once at construction (efficient)

### 🔧 Automatic Tool Execution
- LLM decides which tools to use
- Tools are executed automatically
- Results fed back to LLM for reasoning

### ⚡ Performance Optimized
- Schema injected at start, not per-request
- No extra LLM call for structured output
- Smart retry: only if parsing fails (max 1)

### 📊 Execution Statistics
All examples provide detailed statistics:
- Token usage (input/output/total)
- Tool execution counts
- Cost tracking (if configured)
- Conversation history

### 🛡️ Error Handling
- Structured error messages
- Optional stop-on-error behavior
- Parse error detection and reporting

## Configuration Options

```go
agent, _ := react.New[T](
    baseClient,
    react.WithMaxIterations(10),      // Max tool execution loops
    react.WithStopOnError(true),      // Stop on first tool error
    react.WithSysPromptAnnotation(false), // Disable ReAct prompt hints
)
```

## Understanding the Flow

1. **Schema Injection** → Schema added to system prompt at construction
2. **User Prompt** → Sent to LLM
3. **LLM Response** → May include tool calls
4. **Tool Execution** → Tools run automatically
5. **Results to LLM** → Process tool outputs
6. **Repeat 3-5** → Until LLM has final answer
7. **Parse & Return** → Automatic parsing into type `T`
8. **Retry on Failure** → If parsing fails, request JSON explicitly and retry once

## When to Use

### ✅ Use Type-Safe ReAct[T] When:
- You need tools + reasoning + structured output
- Output format is well-defined
- Type safety is important
- You want automatic schema enforcement

### ✅ Use Untyped ReAct[string] When:
- Prototyping or exploring
- Output structure varies by query
- Don't need structured parsing
- Want simple text responses

## Related Examples

- **`examples/layer3/bravesearch-react/`** - ReAct with web search
- **`examples/layer2/cost_tracking/`** - Cost tracking with ReAct
- **`examples/layer2/structured_output/`** - Simple structured output (no tools)

## Learn More

- **Architecture Guide**: `ARCHITECTURE.md` - Type-Safe ReAct section
- **Changelog**: `CHANGELOG.md` - Recent changes and optimizations
- **API Documentation**: See godoc comments in `patterns/react/react.go`

## Common Patterns

### Untyped Text Output (Default)
```go
// Using string for simple text responses
agent := react.New[string](baseClient)
```

### Multiple Tools
```go
baseClient, _ := client.New(
    provider,
    client.WithTools(calculator, search, webFetch),
    // ... other options
)
```

### Custom Types with Validation
```go
type Result struct {
    Score float64 `json:"score" jsonschema:"required,minimum=0,maximum=100"`
    Grade string  `json:"grade" jsonschema:"required,enum=A|B|C|D|F"`
}
```

### Access Statistics
```go
result, _ := agent.Execute(ctx, prompt)

fmt.Printf("Tokens: %d\n", result.TotalUsage.TotalTokens)
fmt.Printf("Cost: $%.4f\n", result.TotalCost())
fmt.Printf("Tools: %v\n", result.ToolCallStats)
```

## Tips

1. **Define Clear Structures**: Use descriptive field names and jsonschema tags
2. **Use Enums**: Constrain string fields for better LLM compliance
3. **Keep It Simple**: Flat structures work better than deep nesting
4. **Monitor Costs**: Use `result.TotalCost()` to track expenses
5. **Set Reasonable Iterations**: Default is 10, adjust based on task complexity

## Troubleshooting

**Parse errors?**
- Check your struct tags match LLM output
- Add `jsonschema:"required"` to mandatory fields
- Use simpler structures if parsing fails
- Note: Automatic retry happens once if initial parse fails

**Too many iterations?**
- Reduce `WithMaxIterations()` value
- Simplify the task
- Check tool error messages in logs

**Unexpected tool usage?**
- Review system prompt
- Check tool descriptions
- Use `WithEnrichSystemPromptWithToolsDescriptions()` for better guidance