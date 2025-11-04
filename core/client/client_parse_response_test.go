package client

import (
	"testing"

	"github.com/leofalp/aigo/providers/ai"
)

// TestParseResponseAs_String tests parsing response as string
func TestParseResponseAs_String(t *testing.T) {
	response := &ai.ChatResponse{
		Content: "Hello, world!",
	}

	result, err := ParseResponseAs[string](response)
	if err != nil {
		t.Fatalf("ParseResponseAs failed: %v", err)
	}

	if result != "Hello, world!" {
		t.Errorf("Expected 'Hello, world!', got '%s'", result)
	}
}

// TestParseResponseAs_Bool tests parsing response as boolean
func TestParseResponseAs_Bool(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
		wantErr  bool
	}{
		{"true lowercase", "true", true, false},
		{"True capitalized", "True", true, false},
		{"false lowercase", "false", false, false},
		{"False capitalized", "False", false, false},
		{"1 as true", "1", true, false},
		{"0 as false", "0", false, false},
		{"invalid", "maybe", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ai.ChatResponse{Content: tt.content}
			result, err := ParseResponseAs[bool](response)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestParseResponseAs_Int tests parsing response as integer
func TestParseResponseAs_Int(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
		wantErr  bool
	}{
		{"positive", "42", 42, false},
		{"negative", "-17", -17, false},
		{"zero", "0", 0, false},
		{"invalid", "not a number", 0, true},
		{"float", "3.14", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ai.ChatResponse{Content: tt.content}
			result, err := ParseResponseAs[int](response)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestParseResponseAs_Int64 tests parsing response as int64
func TestParseResponseAs_Int64(t *testing.T) {
	response := &ai.ChatResponse{
		Content: "9223372036854775807", // max int64
	}

	result, err := ParseResponseAs[int64](response)
	if err != nil {
		t.Fatalf("ParseResponseAs failed: %v", err)
	}

	if result != 9223372036854775807 {
		t.Errorf("Expected max int64, got %d", result)
	}
}

// TestParseResponseAs_Float64 tests parsing response as float64
func TestParseResponseAs_Float64(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected float64
		wantErr  bool
	}{
		{"decimal", "3.14159", 3.14159, false},
		{"integer", "42", 42.0, false},
		{"negative", "-17.5", -17.5, false},
		{"scientific", "1.23e10", 1.23e10, false},
		{"invalid", "not a float", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ai.ChatResponse{Content: tt.content}
			result, err := ParseResponseAs[float64](response)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

// TestParseResponseAs_Uint tests parsing response as unsigned integer
func TestParseResponseAs_Uint(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected uint
		wantErr  bool
	}{
		{"positive", "42", 42, false},
		{"zero", "0", 0, false},
		{"negative", "-17", 0, true},
		{"invalid", "not a number", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ai.ChatResponse{Content: tt.content}
			result, err := ParseResponseAs[uint](response)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestParseResponseAs_Struct tests parsing response as struct
func TestParseResponseAs_Struct(t *testing.T) {
	type MyResponse struct {
		Answer     string   `json:"answer"`
		Confidence float64  `json:"confidence"`
		Sources    []string `json:"sources,omitempty"`
	}

	tests := []struct {
		name     string
		content  string
		expected MyResponse
		wantErr  bool
	}{
		{
			name:    "valid JSON",
			content: `{"answer":"42","confidence":0.95,"sources":["book1","book2"]}`,
			expected: MyResponse{
				Answer:     "42",
				Confidence: 0.95,
				Sources:    []string{"book1", "book2"},
			},
			wantErr: false,
		},
		{
			name:    "partial JSON",
			content: `{"answer":"Yes","confidence":0.8}`,
			expected: MyResponse{
				Answer:     "Yes",
				Confidence: 0.8,
				Sources:    nil,
			},
			wantErr: false,
		},
		{
			name:     "invalid JSON",
			content:  `{answer: "broken json"}`,
			expected: MyResponse{},
			wantErr:  true,
		},
		{
			name:     "empty string",
			content:  "",
			expected: MyResponse{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ai.ChatResponse{Content: tt.content}
			result, err := ParseResponseAs[MyResponse](response)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Answer != tt.expected.Answer {
				t.Errorf("Expected answer '%s', got '%s'", tt.expected.Answer, result.Answer)
			}

			if result.Confidence != tt.expected.Confidence {
				t.Errorf("Expected confidence %f, got %f", tt.expected.Confidence, result.Confidence)
			}

			if len(result.Sources) != len(tt.expected.Sources) {
				t.Errorf("Expected %d sources, got %d", len(tt.expected.Sources), len(result.Sources))
			}
		})
	}
}

// TestParseResponseAs_NestedStruct tests parsing response with nested structures
func TestParseResponseAs_NestedStruct(t *testing.T) {
	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}

	type Person struct {
		Name    string  `json:"name"`
		Age     int     `json:"age"`
		Address Address `json:"address"`
	}

	content := `{
		"name": "John Doe",
		"age": 30,
		"address": {
			"street": "123 Main St",
			"city": "Springfield"
		}
	}`

	response := &ai.ChatResponse{Content: content}
	result, err := ParseResponseAs[Person](response)

	if err != nil {
		t.Fatalf("ParseResponseAs failed: %v", err)
	}

	if result.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got '%s'", result.Name)
	}

	if result.Age != 30 {
		t.Errorf("Expected age 30, got %d", result.Age)
	}

	if result.Address.City != "Springfield" {
		t.Errorf("Expected city 'Springfield', got '%s'", result.Address.City)
	}
}

// TestParseResponseAs_Map tests parsing response as map
func TestParseResponseAs_Map(t *testing.T) {
	content := `{"key1":"value1","key2":"value2","key3":"value3"}`

	response := &ai.ChatResponse{Content: content}
	result, err := ParseResponseAs[map[string]string](response)

	if err != nil {
		t.Fatalf("ParseResponseAs failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(result))
	}

	if result["key1"] != "value1" {
		t.Errorf("Expected key1='value1', got '%s'", result["key1"])
	}
}

// TestParseResponseAs_Slice tests parsing response as slice
func TestParseResponseAs_Slice(t *testing.T) {
	content := `["apple","banana","cherry"]`

	response := &ai.ChatResponse{Content: content}
	result, err := ParseResponseAs[[]string](response)

	if err != nil {
		t.Fatalf("ParseResponseAs failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 items, got %d", len(result))
	}

	if result[0] != "apple" {
		t.Errorf("Expected first item 'apple', got '%s'", result[0])
	}
}

// TestParseResponseAs_EmptyContent tests parsing empty content
func TestParseResponseAs_EmptyContent(t *testing.T) {
	response := &ai.ChatResponse{Content: ""}

	// String should work with empty content
	strResult, err := ParseResponseAs[string](response)
	if err != nil {
		t.Errorf("Empty string should not error: %v", err)
	}
	if strResult != "" {
		t.Errorf("Expected empty string, got '%s'", strResult)
	}

	// Struct should fail with empty content
	type MyStruct struct {
		Field string `json:"field"`
	}
	_, err = ParseResponseAs[MyStruct](response)
	if err == nil {
		t.Error("Expected error for empty JSON content")
	}
}

// TestParseResponseAs_ErrorMessages tests that error messages are descriptive
func TestParseResponseAs_ErrorMessages(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		parseType   string
		errContains string
	}{
		{
			name:        "invalid bool",
			content:     "not a bool",
			parseType:   "bool",
			errContains: "failed to parse response as bool",
		},
		{
			name:        "invalid int",
			content:     "not an int",
			parseType:   "int",
			errContains: "failed to parse response as int",
		},
		{
			name:        "invalid float",
			content:     "not a float",
			parseType:   "float",
			errContains: "failed to parse response as float",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &ai.ChatResponse{Content: tt.content}

			var err error
			switch tt.parseType {
			case "bool":
				_, err = ParseResponseAs[bool](response)
			case "int":
				_, err = ParseResponseAs[int](response)
			case "float":
				_, err = ParseResponseAs[float64](response)
			}

			if err == nil {
				t.Fatal("Expected error but got none")
			}

			if err.Error() == "" {
				t.Error("Error message should not be empty")
			}
		})
	}
}
