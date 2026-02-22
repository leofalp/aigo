// Package main demonstrates direct use of the Gemini provider (Layer 1) without the
// higher-level client abstraction. It shows a grounded chat message using Google Search
// and a structured-output request with an inline JSON schema.
// Requires the GEMINI_API_KEY environment variable.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/ai/gemini"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	provider := gemini.New()
	ctx := context.Background()

	// Example 1: Simple message with Google Search grounding
	response, err := provider.SendMessage(ctx, ai.ChatRequest{
		Model:        "gemini-flash-lite-latest",
		SystemPrompt: "You are a helpful assistant. Be concise.",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "What are the latest news about AI?"},
		},
		Tools: []ai.ToolDescription{
			{Name: ai.ToolGoogleSearch}, // Enable Google Search grounding
		},
	})
	if err != nil {
		slog.Error("Error", "error", err)
		os.Exit(1)
	}
	fmt.Printf("With Google Search:\n%s\n\n", response.Content)

	// Example 2: Structured output with JSON schema
	response2, err := provider.SendMessage(ctx, ai.ChatRequest{
		Model: "gemini-flash-lite-latest",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Give me info about Rome, Italy"},
		},
		ResponseFormat: &ai.ResponseFormat{
			OutputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"city":       {Type: "string"},
					"country":    {Type: "string"},
					"population": {Type: "integer"},
					"famous_for": {Type: "array", Items: &jsonschema.Schema{Type: "string"}},
				},
			},
		},
	})
	if err != nil {
		slog.Error("Error", "error", err)
		os.Exit(1)
	}
	fmt.Printf("Structured Output:\n%s\n", response2.Content)
}
