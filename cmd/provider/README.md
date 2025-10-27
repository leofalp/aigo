# Provider System

This directory contains the generic provider interface and implementations for various LLM providers.

## Architecture

The provider system separates the LLM provider logic (API keys, models, HTTP calls) from the business logic (schema generation, tool management, etc.). This allows for:

1. **Modularity**: Easy to add new providers
2. **Testability**: Mock providers for testing
3. **Flexibility**: Switch between providers without changing business logic
4. **Maintainability**: Clean separation of concerns

## Structure

```
cmd/provider/
├── provider.go           # Generic Provider interface and types
└── openai/
    └── openai.go         # OpenAI provider implementation
```

## Provider Interface

All providers must implement the `Provider` interface:

```go
type Provider interface {
    SendMessage(ctx context.Context, request ChatRequest) (*ChatResponse, error)
    GetModelName() string
    SetModel(model string)
    
    // Common builder methods
    WithAPIKey(apiKey string) Provider
    WithModel(model string) Provider
    WithBaseURL(baseURL string) Provider
}
```

## Builder Pattern

All providers use a builder pattern for configuration. Each provider has:
- A `New<Provider>()` constructor with sensible defaults
- Builder methods (`WithAPIKey`, `WithModel`, `WithBaseURL`) that return the provider for chaining

### Default Values

Common defaults across all providers:
- **API Key**: Read from environment variable (e.g., `OPENAI_API_KEY`)
- **Base URL**: Provider's official API endpoint
- **Model**: Provider's recommended default model

## Types

### Message
Represents a single message in a conversation.

```go
type Message struct {
    Role    string `json:"role"`    // "system", "user", or "assistant"
    Content string `json:"content"`
}
```

### ToolDefinition
Represents a tool/function that can be called by the LLM.

```go
type ToolDefinition struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Parameters  interface{} `json:"parameters"` // JSON Schema
}
```

### ChatRequest
Request structure for sending messages.

```go
type ChatRequest struct {
    Messages []Message        `json:"messages"`
    Tools    []ToolDefinition `json:"tools,omitempty"`
}
```

### ChatResponse
Response structure from the LLM.

```go
type ChatResponse struct {
    Content      string     `json:"content"`
    ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
    FinishReason string     `json:"finish_reason"`
}
```

## Implemented Providers

### OpenAI Provider

Located in `openai/openai.go`, this provider supports:
- OpenAI API
- OpenAI-compatible APIs (OpenRouter, LocalAI, etc.)

#### Usage

```go
import (
    "aigo/cmd/provider/openai"
    "context"
)

// Example 1: Use defaults (API key from OPENAI_API_KEY env var)
provider := openai.NewOpenAIProvider()

// Example 2: Configure with builder pattern
provider := openai.NewOpenAIProvider().
    WithAPIKey("sk-...").
    WithModel("gpt-4o-mini")

// Example 3: Chain multiple configurations
provider := openai.NewOpenAIProvider().
    WithAPIKey("sk-...").
    WithModel("gpt-4o").
    WithBaseURL("https://api.openai.com/v1")

// Example 4: Use with OpenRouter
provider := openai.NewOpenAIProvider().
    WithAPIKey("your-openrouter-key").
    WithModel("openai/gpt-4o-mini").
    WithBaseURL("https://openrouter.ai/api/v1")

// Send a message
response, err := provider.SendMessage(context.Background(), provider.ChatRequest{
    Messages: []provider.Message{
        {Role: "user", Content: "Hello!"},
    },
})
```

#### Default Values

- **API Key**: `OPENAI_API_KEY` environment variable
- **Model**: `gpt-4o-mini`
- **Base URL**: `https://api.openai.com/v1`

#### Builder Methods

- `WithAPIKey(apiKey string)`: Set the API key
- `WithModel(model string)`: Set the model name
- `WithBaseURL(baseURL string)`: Set the base URL (for OpenAI-compatible APIs)
- `WithHTTPClient(client *http.Client)`: Set a custom HTTP client

## Adding a New Provider

To add a new provider (e.g., Anthropic, Cohere):

1. Create a new directory: `cmd/provider/anthropic/`
2. Create the provider file: `anthropic.go`
3. Implement the `Provider` interface
4. Follow the builder pattern

Example skeleton:

```go
package anthropic

import (
    "aigo/cmd/provider"
    "context"
    "os"
)

const (
    DefaultBaseURL = "https://api.anthropic.com/v1"
    DefaultModel   = "claude-3-5-sonnet-20241022"
)

type AnthropicProvider struct {
    apiKey  string
    model   string
    baseURL string
}

func NewAnthropicProvider() *AnthropicProvider {
    return &AnthropicProvider{
        apiKey:  os.Getenv("ANTHROPIC_API_KEY"),
        model:   DefaultModel,
        baseURL: DefaultBaseURL,
    }
}

func (p *AnthropicProvider) WithAPIKey(apiKey string) provider.Provider {
    p.apiKey = apiKey
    return p
}

func (p *AnthropicProvider) WithModel(model string) provider.Provider {
    p.model = model
    return p
}

func (p *AnthropicProvider) WithBaseURL(baseURL string) provider.Provider {
    p.baseURL = baseURL
    return p
}

func (p *AnthropicProvider) SendMessage(ctx context.Context, request provider.ChatRequest) (*provider.ChatResponse, error) {
    // Implementation here
}

func (p *AnthropicProvider) GetModelName() string {
    return p.model
}

func (p *AnthropicProvider) SetModel(model string) {
    p.model = model
}
```

## Environment Variables

Each provider should respect standard environment variables:

- OpenAI: `OPENAI_API_KEY`
- Anthropic: `ANTHROPIC_API_KEY`
- Cohere: `COHERE_API_KEY`
- Custom base URL: `<PROVIDER>_BASE_URL` (optional)

## Examples

See `examples/providerExample/main.go` for complete usage examples including:
- Using default values
- Builder pattern configuration
- Tool/function calling
- OpenAI-compatible APIs

## Future Enhancements

Potential future additions to the `Provider` interface:

- Streaming support: `SendMessageStream(ctx, request) (Stream, error)`
- Token counting: `CountTokens(messages) (int, error)`
- Model listing: `ListModels() ([]string, error)`
- Rate limiting: Built-in rate limiting support
- Retry logic: Configurable retry mechanisms
- Cost tracking: Track API usage costs
- Embeddings: `CreateEmbedding(ctx, input) ([]float64, error)`

