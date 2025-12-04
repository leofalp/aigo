# AIGO - AI Go Framework

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A lightweight, modular, and extensible Go framework for building AI applications. AIGO provides a flexible 3-layer architecture that lets you choose the right level of abstraction for your needs—from low-level provider control to high-level agentic patterns. Built with minimal dependencies, it integrates seamlessly into existing Go projects while offering powerful features like type-safe structured outputs, automatic cost tracking, and comprehensive observability.

## Features

- **3-Layer Architecture** - Independent layers for providers, core orchestration, and high-level patterns
- **Type-Safe Structured Output** - Generic-based responses with automatic JSON parsing and schema generation
- **Agentic Patterns** - ReAct, RAG, and chain-based workflows with tool execution loops
- **Cost Tracking** - Track and optimize spending across models, tools, and infrastructure
- **Observability** - Built-in tracing, metrics, and logging with zero overhead when disabled
- **Provider Flexibility** - Support for any OpenAI-compatible API
- **Extensible Tools** - Built-in tools for calculations, web search, and scraping, with easy custom tool creation
- **Memory Management** - Thread-safe conversation history with pluggable storage backends

## Architecture Overview

AIGO uses a 3-layer architecture where each layer is independent and optional:

```
┌─────────────────────────────────────────────────┐
│  Layer 3: patterns/                             │
│  High-level AI patterns (ReAct, RAG, Chain)     │
└─────────────────────────────────────────────────┘
                    ↓ require
┌─────────────────────────────────────────────────┐
│  Layer 2: core/                                 │
│  Shared business logic (Client, Tools, etc.)    │
└─────────────────────────────────────────────────┘
                    ↓ using
┌─────────────────────────────────────────────────┐
│  Layer 1: providers/                            │
│  Protocol implementations (OpenAI, Redis, etc.) │
└─────────────────────────────────────────────────┘
```

**Choose your layer based on your needs:**

- **Layer 1** - Maximum control and custom workflows
- **Layer 2** - Managed sessions, tools, and middleware
- **Layer 3** - Quick start with proven patterns

Read the [Architecture Guide](ARCHITECTURE.md) for complete details.

## Installation

```bash
go get github.com/leofalp/aigo
```

## Dependencies

AIGO is built with a minimal dependency philosophy. The core framework relies primarily on the Go standard library, with only a few carefully selected external packages. This keeps the library lightweight and reduces potential conflicts in your projects.

Thanks to Go's module graph pruning, you only download the dependencies you actually use. Each component lives in its own package with isolated dependencies—importing just the core client won't pull in tool-specific libraries, and using a single provider won't download dependencies for others. This modular design ensures your binary stays lean.

## Configuration

Create a `.env` file from the template:

```bash
cp .env.example .env
```

Minimum required variables:

```bash
OPENAI_API_KEY=your-api-key-here
AIGO_DEFAULT_LLM_MODEL=your-model-name
```

See [.env.example](.env.example) for all available configuration options.

## Documentation

### Core Documentation

- **[Architecture Guide](ARCHITECTURE.md)** - Detailed explanation of the 3-layer design
- **[Cost Tracking](core/cost/README.md)** - Track and optimize model and tool costs
- **[Observability](providers/observability/README.md)** - Tracing, metrics, and structured logging

### Components

- **[Providers](providers/)** - Pure I/O implementations for LLM APIs, memory, tools, and observability
- **[Core](core/)** - Shared business logic including client orchestration, cost tracking, and parsing
- **[Patterns](patterns/)** - High-level AI patterns like ReAct agents
- **[Tools](providers/tool/)** - Built-in and custom tools (calculator, web search, scraping, etc.). Custom tools can be easily created by implementing the `Tool` interface.
- **[Examples](examples/)** - Complete working examples for each layer

## Examples

### Basic Usage (Layer 2)

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/leofalp/aigo/core/client"
    "github.com/leofalp/aigo/providers/ai/openai"
    "github.com/leofalp/aigo/providers/memory/inmemory"
)

func main() {
    // Create client with memory
    aiClient, err := client.New(
        openai.New(),
        client.WithMemory(inmemory.New()),
        client.WithSystemPrompt("You are a helpful assistant."),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Send message
    ctx := context.Background()
    resp, err := aiClient.SendMessage(ctx, "What is the capital of France?")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(resp.Content)
}
```

### Structured Output (Layer 2)

```go
type ProductReview struct {
    ProductName string `json:"product_name" jsonschema:"required"`
    Rating      int    `json:"rating" jsonschema:"required"`
    Summary     string `json:"summary" jsonschema:"required"`
}

// Create type-safe client
reviewClient, _ := client.NewStructured[ProductReview](
    openai.New(),
    client.WithMemory(inmemory.New()),
)

// Get structured response
resp, _ := reviewClient.SendMessage(ctx, "Analyze this review: ...")
fmt.Printf("Product: %s, Rating: %d/5\n", resp.Data.ProductName, resp.Data.Rating)
```

### ReAct Agent with Tools (Layer 3)

```go
import (
    "github.com/leofalp/aigo/patterns/react"
    "github.com/leofalp/aigo/providers/tool/calculator"
)

// Create base client with tools
baseClient, _ := client.New(
    openai.New(),
    client.WithMemory(inmemory.New()),
    client.WithTools(calculator.NewCalculatorTool()),
)

// Create type-safe ReAct agent
agent, _ := react.New[string](baseClient)

// Execute with automatic tool loop
result, _ := agent.Execute(ctx, "What is 42 * 17?")
fmt.Println(*result.Data)
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

Built by [Leandro Frola](https://github.com/leofalp) with [friends](https://github.com/leofalp/aigo/graphs/contributors)