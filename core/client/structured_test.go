package client

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/memory/inmemory"
)

// TestStructuredClient_SendMessage tests basic structured client functionality
func TestStructuredClient_SendMessage(t *testing.T) {
	type TestResponse struct {
		Answer     string `json:"answer" jsonschema:"required"`
		Confidence int    `json:"confidence" jsonschema:"required"`
	}

	// Create mock provider
	mockProvider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			// Verify that ResponseFormat is set correctly
			if request.ResponseFormat == nil {
				t.Error("Expected ResponseFormat to be set")
			}
			if request.ResponseFormat.Type != "json_schema" {
				t.Errorf("Expected ResponseFormat.Type to be 'json_schema', got '%s'", request.ResponseFormat.Type)
			}
			if request.ResponseFormat.OutputSchema == nil {
				t.Error("Expected ResponseFormat.OutputSchema to be set")
			}

			// Return a valid JSON response matching the schema
			responseData := TestResponse{
				Answer:     "The answer is 42",
				Confidence: 95,
			}
			jsonBytes, _ := json.Marshal(responseData)

			return &ai.ChatResponse{
				Id:           "test-response-1",
				Model:        "test-model",
				Content:      string(jsonBytes),
				FinishReason: "stop",
				Usage: &ai.Usage{
					TotalTokens: 100,
				},
			}, nil
		},
	}

	// Create base client
	baseClient, err := NewClient(mockProvider)
	if err != nil {
		t.Fatalf("Failed to create base client: %v", err)
	}

	// Create structured client
	structuredClient := NewStructuredClient[TestResponse](baseClient)

	// Send message
	resp, err := structuredClient.SendMessage(context.Background(), "What is the meaning of life?")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify parsed data
	if resp.Data == nil {
		t.Fatal("Expected Data to be non-nil")
	}
	if resp.Data.Answer != "The answer is 42" {
		t.Errorf("Expected Answer='The answer is 42', got '%s'", resp.Data.Answer)
	}
	if resp.Data.Confidence != 95 {
		t.Errorf("Expected Confidence=95, got %d", resp.Data.Confidence)
	}

	// Verify raw response is accessible
	if resp.Usage.TotalTokens != 100 {
		t.Errorf("Expected TotalTokens=100, got %d", resp.Usage.TotalTokens)
	}
}

// TestStructuredClient_ContinueConversation tests structured continue conversation
func TestStructuredClient_ContinueConversation(t *testing.T) {
	type ConversationResponse struct {
		Message string `json:"message" jsonschema:"required"`
	}

	callCount := 0
	mockProvider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			callCount++

			// Verify schema is applied
			if request.ResponseFormat == nil || request.ResponseFormat.OutputSchema == nil {
				t.Error("Expected OutputSchema to be set")
			}

			responseData := ConversationResponse{
				Message: "Response " + string(rune('0'+callCount)),
			}
			jsonBytes, _ := json.Marshal(responseData)

			return &ai.ChatResponse{
				Id:           "test-response",
				Content:      string(jsonBytes),
				FinishReason: "stop",
			}, nil
		},
	}

	// Create client with memory for stateful conversation
	baseClient, err := NewClient(
		mockProvider,
		WithMemory(inmemory.New()),
	)
	if err != nil {
		t.Fatalf("Failed to create base client: %v", err)
	}

	structuredClient := NewStructuredClient[ConversationResponse](baseClient)

	// First message
	resp1, err := structuredClient.SendMessage(context.Background(), "Hello")
	if err != nil {
		t.Fatalf("First SendMessage failed: %v", err)
	}
	if resp1.Data.Message != "Response 1" {
		t.Errorf("Expected Message='Response 1', got '%s'", resp1.Data.Message)
	}

	// Continue conversation
	resp2, err := structuredClient.ContinueConversation(context.Background())
	if err != nil {
		t.Fatalf("ContinueConversation failed: %v", err)
	}
	if resp2.Data.Message != "Response 2" {
		t.Errorf("Expected Message='Response 2', got '%s'", resp2.Data.Message)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls to provider, got %d", callCount)
	}
}

// TestStructuredClient_SchemaOverride tests that per-request schema can override default
func TestStructuredClient_SchemaOverride(t *testing.T) {
	type DefaultResponse struct {
		Value string `json:"value"`
	}

	type OverrideResponse struct {
		Different string `json:"different"`
	}

	mockProvider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			// Return JSON that could match either schema
			return &ai.ChatResponse{
				Id:           "test",
				Content:      `{"value":"default","different":"override"}`,
				FinishReason: "stop",
			}, nil
		},
	}

	baseClient, err := NewClient(mockProvider)
	if err != nil {
		t.Fatalf("Failed to create base client: %v", err)
	}

	structuredClient := NewStructuredClient[DefaultResponse](baseClient)

	// Normal call uses default schema
	resp1, err := structuredClient.SendMessage(context.Background(), "test")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if resp1.Data.Value != "default" {
		t.Errorf("Expected Value='default', got '%s'", resp1.Data.Value)
	}

	// Override with different schema
	overrideSchema := jsonschema.GenerateJSONSchema[OverrideResponse]()
	resp2, err := structuredClient.SendMessage(
		context.Background(),
		"test",
		WithOutputSchema(overrideSchema),
	)
	if err != nil {
		t.Fatalf("SendMessage with override failed: %v", err)
	}

	// Parse should still work with default type (this is expected behavior)
	// The schema tells the LLM what to produce, but parsing uses the client's type
	if resp2.Data.Value != "default" {
		t.Errorf("Expected Value='default', got '%s'", resp2.Data.Value)
	}
}

// TestStructuredClient_Base tests accessing the underlying base client
func TestStructuredClient_Base(t *testing.T) {
	type TestResponse struct {
		Data string `json:"data"`
	}

	mockProvider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			return &ai.ChatResponse{
				Id:           "test",
				Content:      `{"data":"test"}`,
				FinishReason: "stop",
			}, nil
		},
	}

	memory := inmemory.New()
	baseClient, err := NewClient(
		mockProvider,
		WithMemory(memory),
		WithSystemPrompt("Test prompt"),
	)
	if err != nil {
		t.Fatalf("Failed to create base client: %v", err)
	}

	structuredClient := NewStructuredClient[TestResponse](baseClient)

	// Access base client
	base := structuredClient.Base()
	if base == nil {
		t.Fatal("Expected Base() to return non-nil client")
	}

	// Verify base client has expected configuration
	if base.Memory() != memory {
		t.Error("Expected Base() to return client with same memory")
	}
}

// TestStructuredClient_Schema tests accessing the schema
func TestStructuredClient_Schema(t *testing.T) {
	type TestResponse struct {
		Field string `json:"field" jsonschema:"required,description=A test field"`
	}

	mockProvider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			return &ai.ChatResponse{
				Id:           "test",
				Content:      `{"field":"value"}`,
				FinishReason: "stop",
			}, nil
		},
	}

	baseClient, err := NewClient(mockProvider)
	if err != nil {
		t.Fatalf("Failed to create base client: %v", err)
	}

	structuredClient := NewStructuredClient[TestResponse](baseClient)

	// Access schema
	schema := structuredClient.Schema()
	if schema == nil {
		t.Fatal("Expected Schema() to return non-nil schema")
	}

	// Verify schema structure
	if schema.Type != "object" {
		t.Errorf("Expected schema type 'object', got '%s'", schema.Type)
	}
	if schema.Properties == nil {
		t.Fatal("Expected schema to have properties")
	}
	if _, exists := schema.Properties["field"]; !exists {
		t.Error("Expected schema to have 'field' property")
	}
}

// TestStructuredClient_ParseError tests error handling for invalid JSON
func TestStructuredClient_ParseError(t *testing.T) {
	type TestResponse struct {
		Value int `json:"value"`
	}

	mockProvider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			// Return invalid JSON
			return &ai.ChatResponse{
				Id:           "test",
				Content:      "This is not valid JSON",
				FinishReason: "stop",
			}, nil
		},
	}

	baseClient, err := NewClient(mockProvider)
	if err != nil {
		t.Fatalf("Failed to create base client: %v", err)
	}

	structuredClient := NewStructuredClient[TestResponse](baseClient)

	// Should fail to parse
	_, err = structuredClient.SendMessage(context.Background(), "test")
	if err == nil {
		t.Fatal("Expected SendMessage to return error for invalid JSON")
	}

	// Error should mention parsing failure
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

// TestStructuredClient_ComplexType tests structured client with nested types
func TestStructuredClient_ComplexType(t *testing.T) {
	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}

	type Person struct {
		Name    string  `json:"name" jsonschema:"required"`
		Age     int     `json:"age" jsonschema:"required"`
		Address Address `json:"address" jsonschema:"required"`
	}

	mockProvider := &mockProvider{
		sendMessageFunc: func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			responseData := Person{
				Name: "John Doe",
				Age:  30,
				Address: Address{
					Street: "123 Main St",
					City:   "New York",
				},
			}
			jsonBytes, _ := json.Marshal(responseData)

			return &ai.ChatResponse{
				Id:           "test",
				Content:      string(jsonBytes),
				FinishReason: "stop",
			}, nil
		},
	}

	baseClient, err := NewClient(mockProvider)
	if err != nil {
		t.Fatalf("Failed to create base client: %v", err)
	}

	structuredClient := NewStructuredClient[Person](baseClient)

	resp, err := structuredClient.SendMessage(context.Background(), "Get person info")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify nested structure
	if resp.Data.Name != "John Doe" {
		t.Errorf("Expected Name='John Doe', got '%s'", resp.Data.Name)
	}
	if resp.Data.Age != 30 {
		t.Errorf("Expected Age=30, got %d", resp.Data.Age)
	}
	if resp.Data.Address.City != "New York" {
		t.Errorf("Expected City='New York', got '%s'", resp.Data.Address.City)
	}
}
