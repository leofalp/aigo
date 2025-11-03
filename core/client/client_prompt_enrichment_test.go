package client

import (
	"context"
	"strings"
	"testing"

	"github.com/leofalp/aigo/providers/ai"
)

// Test that enrichSystemPromptWithTools correctly appends tool information
func TestEnrichSystemPromptWithTools(t *testing.T) {
	basePrompt := "You are a helpful assistant."

	tools := []ai.ToolDescription{
		{
			Name:        "Calculator",
			Description: "Performs basic arithmetic operations",
			Parameters:  nil,
		},
		{
			Name:        "WebSearch",
			Description: "Searches the web for information",
			Parameters:  nil,
		},
	}

	enriched := enrichSystemPromptWithTools(basePrompt, tools)

	// Verify base prompt is included
	if !strings.Contains(enriched, basePrompt) {
		t.Error("Enriched prompt should contain the base prompt")
	}

	// Verify tools section is added
	if !strings.Contains(enriched, "## Available Tools") {
		t.Error("Enriched prompt should contain 'Available Tools' section")
	}

	// Verify each tool is mentioned
	for _, tool := range tools {
		if !strings.Contains(enriched, tool.Name) {
			t.Errorf("Enriched prompt should contain tool name '%s'", tool.Name)
		}
		if !strings.Contains(enriched, tool.Description) {
			t.Errorf("Enriched prompt should contain tool description for '%s'", tool.Name)
		}
	}

	// Verify guidance is included
	if !strings.Contains(enriched, "function calling") {
		t.Error("Enriched prompt should contain function calling guidance")
	}
}

func TestEnrichSystemPromptWithTools_EmptyTools(t *testing.T) {
	basePrompt := "You are a helpful assistant."
	tools := []ai.ToolDescription{}

	enriched := enrichSystemPromptWithTools(basePrompt, tools)

	// Should return base prompt unchanged when no tools
	if enriched != basePrompt {
		t.Error("Expected enriched prompt to equal base prompt when no tools provided")
	}
}

func TestEnrichSystemPromptWithTools_NilTools(t *testing.T) {
	basePrompt := "You are a helpful assistant."
	var tools []ai.ToolDescription

	enriched := enrichSystemPromptWithTools(basePrompt, tools)

	// Should return base prompt unchanged when tools is nil
	if enriched != basePrompt {
		t.Error("Expected enriched prompt to equal base prompt when tools is nil")
	}
}

func TestEnrichSystemPromptWithTools_EmptyBasePrompt(t *testing.T) {
	basePrompt := ""
	tools := []ai.ToolDescription{
		{
			Name:        "TestTool",
			Description: "A test tool",
		},
	}

	enriched := enrichSystemPromptWithTools(basePrompt, tools)

	// Should add tools section even with empty base prompt
	if !strings.Contains(enriched, "## Available Tools") {
		t.Error("Enriched prompt should contain tools section even with empty base prompt")
	}
	if !strings.Contains(enriched, "TestTool") {
		t.Error("Enriched prompt should contain tool name")
	}
}

func TestWithEnrichSystemPrompt_Option(t *testing.T) {
	options := &ClientOptions{}

	// Apply the option
	WithEnrichSystemPromptWithToolsDescriptions()(options)
	WithEnrichSystemPromptWithOutputSchema()(options)

	if !options.EnrichSystemPromptWithToolDescr {
		t.Error("WithEnrichSystemPromptWithToolsDescriptions should set EnrichSystemPrompt to true")
	}

	if !options.EnrichSystemPromptWithOutputSchema {
		t.Error("WithEnrichSystemPromptWithToolsDescriptions should set EnrichSystemPrompt to true")
	}
}

func TestNewClient_WithEnrichSystemPrompt_Enabled(t *testing.T) {
	// Setup
	provider := &mockProvider{}
	mockTool := &mockTool{
		name:        "TestTool",
		description: "A tool for testing",
	}

	// Create client with enrichment enabled
	client, err := NewClient[string](
		provider,
		WithSystemPrompt("You are a helpful assistant."),
		WithTools(mockTool),
		WithEnrichSystemPromptWithToolsDescriptions(), // Enable enrichment
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify system prompt was enriched
	if !strings.Contains(client.systemPrompt, "You are a helpful assistant.") {
		t.Error("Client system prompt should contain base prompt")
	}
	if !strings.Contains(client.systemPrompt, "Available Tools") {
		t.Error("Client system prompt should be enriched with tools section")
	}
	if !strings.Contains(client.systemPrompt, "TestTool") {
		t.Error("Client system prompt should contain tool name")
	}
	if !strings.Contains(client.systemPrompt, "A tool for testing") {
		t.Error("Client system prompt should contain tool description")
	}
}

func TestNewClient_WithEnrichSystemPrompt_Disabled(t *testing.T) {
	// Setup
	provider := &mockProvider{}
	mockTool := &mockTool{
		name:        "TestTool",
		description: "A tool for testing",
	}

	basePrompt := "You are a helpful assistant."

	// Create client WITHOUT enrichment (default)
	client, err := NewClient[string](
		provider,
		WithSystemPrompt(basePrompt),
		WithTools(mockTool),
		// No WithEnrichSystemPromptWithToolsDescriptions() - should be disabled by default
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify system prompt was NOT enriched (should be exactly the base prompt)
	if client.systemPrompt != basePrompt {
		t.Error("Client system prompt should not be enriched when enrichment is disabled")
	}
	if strings.Contains(client.systemPrompt, "Available Tools") {
		t.Error("Client system prompt should not contain tools section when enrichment is disabled")
	}
}

func TestNewClient_WithEnrichSystemPrompt_NoTools(t *testing.T) {
	// Setup
	provider := &mockProvider{}
	basePrompt := "You are a helpful assistant."

	// Create client with enrichment enabled but no tools
	client, err := NewClient[string](
		provider,
		WithSystemPrompt(basePrompt),
		WithEnrichSystemPromptWithToolsDescriptions(), // Enable enrichment but no tools
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify system prompt is unchanged (no tools to enrich with)
	if client.systemPrompt != basePrompt {
		t.Error("Client system prompt should not be enriched when no tools are provided")
	}
}

func TestNewClient_WithEnrichSystemPrompt_MultipleTools(t *testing.T) {
	// Setup
	provider := &mockProvider{}
	tool1 := &mockTool{name: "Calculator", description: "Math operations"}
	tool2 := &mockTool{name: "WebSearch", description: "Search the web"}
	tool3 := &mockTool{name: "Database", description: "Query database"}

	// Create client with multiple tools
	client, err := NewClient[string](
		provider,
		WithSystemPrompt("You are a helpful assistant."),
		WithTools(tool1, tool2, tool3),
		WithEnrichSystemPromptWithToolsDescriptions(),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify all tools are mentioned
	allTools := []struct {
		name string
		desc string
	}{
		{"Calculator", "Math operations"},
		{"WebSearch", "Search the web"},
		{"Database", "Query database"},
	}

	for _, tool := range allTools {
		if !strings.Contains(client.systemPrompt, tool.name) {
			t.Errorf("Client system prompt should contain tool name '%s'", tool.name)
		}
		if !strings.Contains(client.systemPrompt, tool.desc) {
			t.Errorf("Client system prompt should contain tool description '%s'", tool.desc)
		}
	}

	// Verify tools are numbered
	if !strings.Contains(client.systemPrompt, "1.") {
		t.Error("Client system prompt should contain numbered list")
	}
	if !strings.Contains(client.systemPrompt, "2.") {
		t.Error("Client system prompt should contain numbered list")
	}
	if !strings.Contains(client.systemPrompt, "3.") {
		t.Error("Client system prompt should contain numbered list")
	}
}

func TestEnrichSystemPromptWithTools_ParameterFormatting(t *testing.T) {
	basePrompt := "Test prompt"
	tools := []ai.ToolDescription{
		{
			Name:        "TestTool",
			Description: "A test tool",
			Parameters:  nil, // Parameters formatting is optional
		},
	}

	enriched := enrichSystemPromptWithTools(basePrompt, tools)

	// Verify tool name and description are included
	if !strings.Contains(enriched, "TestTool") {
		t.Error("Enriched prompt should contain tool name")
	}
	if !strings.Contains(enriched, "A test tool") {
		t.Error("Enriched prompt should contain tool description")
	}
}

func TestNewClient_WithEnrichSystemPrompt_Integration(t *testing.T) {
	// This is an integration test that verifies the enriched prompt
	// is actually used when sending messages

	// Setup mock provider that captures the request
	var capturedSystemPrompt string
	provider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
			capturedSystemPrompt = req.SystemPrompt
			return &ai.ChatResponse{
				Content:      "Response",
				FinishReason: "stop",
			}, nil
		},
	}

	mockTool := &mockTool{
		name:        "Calculator",
		description: "Performs calculations",
	}

	basePrompt := "You are a math assistant."

	// Create client with enrichment
	client, err := NewClient[string](
		provider,
		WithSystemPrompt(basePrompt),
		WithTools(mockTool),
		WithEnrichSystemPromptWithToolsDescriptions(),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Send a message (stateless)
	ctx := context.Background()
	_, err = client.SendMessage(ctx, "What is 2+2?")
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Verify the enriched prompt was sent to the provider
	if capturedSystemPrompt == "" {
		t.Fatal("System prompt was not captured")
	}

	if !strings.Contains(capturedSystemPrompt, basePrompt) {
		t.Error("Captured system prompt should contain base prompt")
	}

	if !strings.Contains(capturedSystemPrompt, "Available Tools") {
		t.Error("Captured system prompt should contain tools section")
	}

	if !strings.Contains(capturedSystemPrompt, "Calculator") {
		t.Error("Captured system prompt should contain tool name")
	}

	if !strings.Contains(capturedSystemPrompt, "Performs calculations") {
		t.Error("Captured system prompt should contain tool description")
	}
}

// mockTool with description field for testing enrichment
type mockTool struct {
	name        string
	description string
	callCount   int
}

func (m *mockTool) ToolInfo() ai.ToolDescription {
	return ai.ToolDescription{
		Name:        m.name,
		Description: m.description,
		Parameters:  nil,
	}
}

func (m *mockTool) Call(ctx context.Context, arguments string) (string, error) {
	m.callCount++
	return `{"result": "success"}`, nil
}
