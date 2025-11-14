package client

import (
	"context"
	"fmt"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/ai"
)

// StructuredClient wraps a base Client and provides type-safe structured output methods.
// The generic parameter T defines the expected response structure for all operations.
//
// StructuredClient automatically:
//   - Generates JSON schema from type T
//   - Applies the schema to all requests via WithOutputSchema
//   - Parses responses into type T
//   - Returns both parsed data and raw response metadata
//
// Example usage:
//
//	type ProductReview struct {
//	    ProductName string `json:"product_name" jsonschema:"required"`
//	    Rating      int    `json:"rating" jsonschema:"required"`
//	    Summary     string `json:"summary" jsonschema:"required"`
//	}
//
//	baseClient, _ := client.NewClient(provider, client.WithMemory(memory))
//	reviewClient := client.NewStructuredClient[ProductReview](baseClient)
//
//	resp, err := reviewClient.SendMessage(ctx, "Analyze this review: ...")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Product: %s\n", resp.Data.ProductName)
//	fmt.Printf("Rating: %d/5\n", resp.Data.Rating)
//	fmt.Printf("Tokens used: %d\n", resp.Raw.Usage.TotalTokens)
type StructuredClient[T any] struct {
	base   *Client
	schema *jsonschema.Schema
}

// NewStructuredClient creates a new structured client wrapper that automatically handles
// structured output for type T. The JSON schema is generated once at creation time
// and reused for all requests.
//
// The base client should be configured with any necessary options (memory, tools, observer, etc.)
// before wrapping it in a StructuredClient.
//
// Example:
//
//	baseClient, _ := client.NewClient(
//	    provider,
//	    client.WithMemory(memory),
//	    client.WithTools(tool1, tool2),
//	)
//	structuredClient := client.NewStructuredClient[MyResponse](baseClient)
func NewStructuredClient[T any](base *Client) *StructuredClient[T] {
	schema := jsonschema.GenerateJSONSchema[T]()
	base.SetDefaultOutputSchema(schema)
	return &StructuredClient[T]{
		base:   base,
		schema: schema,
	}
}

// SendMessage sends a user message to the LLM and returns the parsed structured response.
//
// This method automatically:
//  1. Applies the JSON schema for type T to guide LLM output
//  2. Sends the message using the base client
//  3. Parses the response content into type T
//  4. Returns both parsed data and raw response
//
// Additional SendMessageOptions can be provided to customize the request
// (e.g., to override the schema for a specific request).
//
// The prompt parameter must be non-empty. For continuing a conversation without
// adding a new user message, use ContinueConversation() instead.
//
// Returns StructuredResponse[T] containing:
//   - Data: The parsed structured data of type T
//   - Raw: The original ChatResponse with metadata (usage, reasoning, etc.)
func (sc *StructuredClient[T]) SendMessage(ctx context.Context, prompt string, opts ...SendMessageOption) (*ai.StructuredChatResponse[T], error) {
	// Outcut schema is already set as default in base client and can be overridden by opts
	resp, err := sc.base.SendMessage(ctx, prompt, opts...)
	if err != nil {
		return nil, err
	}

	return sc.parseResponse(resp)
}

// ContinueConversation continues the conversation without adding a new user message,
// and returns the parsed structured response.
//
// This method is useful after tool execution or when you want the LLM to process
// existing messages in memory without adding new user input.
//
// Like SendMessage, this automatically applies the JSON schema for type T and
// parses the response.
//
// Additional SendMessageOptions can be provided to customize the request.
//
// Returns StructuredResponse[T] containing both parsed data and raw response.
func (sc *StructuredClient[T]) ContinueConversation(ctx context.Context, opts ...SendMessageOption) (*ai.StructuredChatResponse[T], error) {
	// Prepend schema option (user can override with their own opts)
	opts = append([]SendMessageOption{WithOutputSchema(sc.schema)}, opts...)

	resp, err := sc.base.ContinueConversation(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return sc.parseResponse(resp)
}

// Base returns the underlying base client for advanced usage.
//
// This can be useful when you need to perform operations that don't require
// structured output, or to access client methods not exposed by StructuredClient.
//
// Example:
//
//	structuredClient := client.NewStructuredClient[MyType](baseClient)
//	memory := structuredClient.Base().Memory()
//	observer := structuredClient.Base().Observer()
func (sc *StructuredClient[T]) Base() *Client {
	return sc.base
}

// Schema returns the JSON schema used for structured output.
//
// This can be useful for debugging or introspection.
func (sc *StructuredClient[T]) Schema() *jsonschema.Schema {
	return sc.schema
}

// parseResponse parses a ChatResponse into a StructuredResponse[T].
// This is an internal helper method used by SendMessage and ContinueConversation.
func (sc *StructuredClient[T]) parseResponse(resp *ai.ChatResponse) (*ai.StructuredChatResponse[T], error) {
	if resp == nil {
		return nil, fmt.Errorf("response is nil")
	}
	data, err := utils.ParseStringAs[T](resp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse structured output: %w", err)
	}

	return &ai.StructuredChatResponse[T]{
		ChatResponse: *resp,
		Data:         &data,
	}, nil
}
