# Architecture

## Overview

`aigo` is structured in three independent layers, each providing increasing levels of abstraction. **You can use any layer directly without depending on the layers above it**, giving you maximum flexibility.

```
┌─────────────────────────────────────────────────┐
│  Layer 3: patterns/                             │
│  High-level AI patterns (ReAct, RAG, Chain)     │
│  Uses: Layer 1 OR Layer 2                       │
└─────────────────────────────────────────────────┘
                    ↓ optional
┌─────────────────────────────────────────────────┐
│  Layer 2: core/                                 │
│  Shared business logic (Session, Tools, etc.)   │
│  Uses: Layer 1                                  │
└─────────────────────────────────────────────────┘
                    ↓ required
┌─────────────────────────────────────────────────┐
│  Layer 1: providers/                            │
│  Protocol implementations (OpenAI, Redis, etc.) │
│  Uses: Nothing (pure I/O)                       │
└─────────────────────────────────────────────────┘
```

---

## Layer 1: Providers (Primitives)

**What it is:** Pure I/O implementations and protocol adapters.

**Responsibility:** Handle communication with external services (APIs, databases, etc.) with zero business logic.

**Examples:**
- `providers/ai/` - LLM providers (OpenAI, Anthropic, Ollama)
- `providers/embedding/` - Embedding models
- `providers/vectorstore/` - Vector databases (Pinecone, Qdrant, Weaviate)
- `providers/memory/` - Storage backends (Redis, PostgreSQL, in-memory)
- `providers/cache/` - Caching systems
- `providers/observability/` - Monitoring and logging

**Interface Example:**

```go
// providers/ai/provider.go
type Provider interface {
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan Chunk, error)
}

// providers/memory/store.go
type Store interface {
    Save(ctx context.Context, key string, data []byte) error
    Load(ctx context.Context, key string) ([]byte, error)
    Delete(ctx context.Context, key string) error
}
```

**Usage:**

```go
import "github.com/leofalp/aigo/providers/ai/openai"

provider := openai.New(apiKey)
resp, err := provider.Complete(ctx, ai.CompletionRequest{
    Messages: []ai.Message{{Role: "user", Content: "Hello!"}},
    Model:    "gpt-4",
})
```

**When to use:**
- ✅ You want maximum control
- ✅ You're building custom orchestration
- ✅ You need specific provider features

---

## Layer 2: Core (Shared Business Logic)

**What it is:** Reusable components with domain logic that orchestrate multiple providers.

**Responsibility:** Provide shared functionality like session management, tool execution, middleware, and context building. **This is NOT a thin wrapper** - it contains real business logic that prevents duplication across patterns.

**Components:**
- `core/client.go` - Main orchestrator that composes all features
- `core/session/` - Conversation memory management (sliding window, summarization)
- `core/tools/` - Tool registry, validation, and execution
- `core/middleware/` - Cross-cutting concerns (retry, cache, rate limiting)
- `core/context/` - Prompt building and token management

**What Layer 2 provides that Layer 1 doesn't:**

| Feature | Layer 1 (Provider) | Layer 2 (Core) |
|---------|-------------------|----------------|
| API Call | ✅ `provider.Complete()` | ✅ Wrapped |
| Conversation History | ❌ | ✅ Auto-managed |
| Tool Calling | ❌ | ✅ Full orchestration |
| Retry/Timeout | ❌ | ✅ Middleware |
| Token Limits | ❌ | ✅ Auto-pruning |
| Multi-provider | ❌ | ✅ Composable |

**Usage:**

```go
import (
    "github.com/leofalp/aigo/core"
    "github.com/leofalp/aigo/providers/ai/openai"
    "github.com/leofalp/aigo/providers/memory/redis"
)

client := core.NewClient(
    openai.New(apiKey),
    core.WithSessionManager(redis.New(redisURL)),
    core.WithRetry(3),
    core.WithCache(cacheProvider),
)

// Session, tools, middleware all handled automatically
resp, err := client.Generate(ctx, sessionID, "Hello!")
```

**When to use:**
- ✅ You need conversation memory
- ✅ You want tool calling support
- ✅ You need retry/cache/observability
- ✅ You're building multiple patterns and want to avoid duplication

---

## Layer 3: Patterns (High-Level Abstractions)

**What it is:** Ready-to-use implementations of common AI patterns.

**Responsibility:** Provide opinionated, production-ready implementations of complex patterns like ReAct agents, RAG pipelines, and chain-of-thought.

**Examples:**
- `patterns/chat/` - Conversational chat interface
- `patterns/react/` - ReAct (Reasoning + Acting) agent
- `patterns/rag/` - Retrieval Augmented Generation
- `patterns/chain/` - Sequential and parallel execution chains
- `patterns/graph/` - State machine / graph-based workflows

**Key Point:** Patterns can use **either Layer 2 OR Layer 1 directly**. They're not forced to use `core.Client`.

**Usage with Layer 2:**

```go
import (
    "github.com/leofalp/aigo/patterns/chat"
    "github.com/leofalp/aigo/providers/ai/openai"
)

// Uses core.Client internally for session management
chat := chat.New(
    openai.New(apiKey),
    chat.WithMemory(redisStore),
)

resp, err := chat.Send(ctx, "Hello!")
```

**Usage with Layer 1 (bypassing Layer 2):**

```go
import (
    "github.com/leofalp/aigo/patterns/react"
    "github.com/leofalp/aigo/providers/ai/openai"
)

// Uses provider directly - you manage everything
agent := react.NewAgent(
    openai.New(apiKey),
    tools,
    react.WithCustomMemory(myMemoryImpl),
)

result, err := agent.Run(ctx, "Complex task")
```

**When to use:**
- ✅ You want quick start with best practices
- ✅ You need proven patterns out-of-the-box
- ✅ You prefer opinionated APIs

---

## Independence Between Layers

### ✅ Layer 1 → Standalone
```go
// Use ONLY providers
provider := openai.New(apiKey)
resp, _ := provider.Complete(ctx, req)
```

### ✅ Layer 2 → Uses Layer 1
```go
// Core uses providers, but you don't need patterns
client := core.NewClient(openai.New(apiKey))
resp, _ := client.Generate(ctx, sessionID, "Hello")
```

### ✅ Layer 3 → Can use Layer 1 OR Layer 2
```go
// Option A: Pattern uses core.Client
chat := chat.New(openai.New(apiKey))

// Option B: Pattern uses provider directly
agent := react.NewAgent(openai.New(apiKey), tools)
```

---

## Design Principles

1. **No Lock-in**: Each layer is optional (except Layer 1)
2. **Composable**: Mix and match components freely
3. **Zero Duplication**: Shared logic lives in Layer 2, not in every pattern
4. **Testable**: Mock at any layer
5. **Go Idioms**: Simple interfaces, explicit dependencies, no magic

---

## Choosing Your Layer

| I want to... | Use Layer |
|-------------|-----------|
| Build custom AI workflows from scratch | Layer 1 |
| Use a provider with specific features | Layer 1 |
| Have conversation memory + tools managed | Layer 2 |
| Get started quickly with chat/agents | Layer 3 |
| Implement a custom agent pattern | Layer 1 or 2 |
| Use retry/cache/observability | Layer 2 |

---

## Example: Same Goal, Different Layers

**Goal:** Chat with memory and tool calling

### Using Layer 1 (Full Control):
```go
provider := openai.New(apiKey)
memStore := redis.New(redisURL)

// You manage everything manually
history, _ := memStore.Load(ctx, sessionID)
messages := append(deserialize(history), userMessage)

resp, _ := provider.Complete(ctx, ai.CompletionRequest{
    Messages: messages,
    Tools:    toolSchemas,
})

// Handle tool calls manually
if resp.ToolCalls != nil {
    for _, tc := range resp.ToolCalls {
        result := executeToolManually(tc)
        // Continue conversation...
    }
}

memStore.Save(ctx, sessionID, serialize(messages))
```

### Using Layer 2 (Managed):
```go
client := core.NewClient(
    openai.New(apiKey),
    core.WithSessionManager(redis.New(redisURL)),
    core.WithTools(tools),
)

// Everything handled automatically
resp, _ := client.Generate(ctx, sessionID, "Hello")
```

### Using Layer 3 (Quickest):
```go
chat := chat.New(
    openai.New(apiKey),
    chat.WithMemory(redis.New(redisURL)),
    chat.WithTools(tools),
)

resp, _ := chat.Send(ctx, "Hello")
```

---

## Philosophy

> "Make the simple things simple, and the complex things possible."

- **Layer 1**: Maximum flexibility, you control everything
- **Layer 2**: Shared components, avoid reinventing the wheel
- **Layer 3**: Quick start, opinionated best practices

Choose the layer that matches your needs. You can always move between layers as your requirements evolve.