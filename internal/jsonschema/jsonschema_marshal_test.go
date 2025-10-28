package jsonschema

import (
	"encoding/json"
	"testing"
)

func TestGenerateJSONSchemaWithValidStruct(t *testing.T) {
	type SampleStruct struct {
		Name  string `json:"name"`
		Age   int    `json:"age"`
		Email string `json:"email"`
	}

	schema, err := GenerateJSONSchema[SampleStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema.Type != "object" {
		t.Errorf("expected schema type to be 'object', got '%s'", schema.Type)
	}

	if len(schema.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(schema.Properties))
	}

	if _, ok := schema.Properties["name"]; !ok {
		t.Error("expected 'name' property to exist")
	}
}

func TestGenerateJSONSchemaWithRecursiveStruct(t *testing.T) {
	type RecursiveStruct struct {
		Name     string             `json:"name"`
		Children []*RecursiveStruct `json:"children"`
	}

	schema, err := GenerateJSONSchema[RecursiveStruct]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if schema.Type != "object" {
		t.Errorf("expected schema type to be 'object', got '%s'", schema.Type)
	}

	if _, ok := schema.Properties["children"]; !ok {
		t.Error("expected 'children' property to exist")
	}

	if schema.Properties["children"].Items.Ref == "" {
		t.Error("expected 'children' property to have a reference")
	}
}

func TestJsonStringWithIndentation(t *testing.T) {
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
		},
	}

	jsonStr, err := schema.JsonString(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if result["type"] != "object" {
		t.Errorf("expected type to be 'object', got '%v'", result["type"])
	}
}

func TestJsonStringWithoutIndentation(t *testing.T) {
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
		},
	}

	jsonStr, err := schema.JsonString(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jsonStr) == 0 {
		t.Error("expected non-empty JSON string")
	}
}

func TestStringMethodReturnsValidJSON(t *testing.T) {
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
		},
	}

	jsonStr := schema.String()
	if len(jsonStr) == 0 {
		t.Error("expected non-empty JSON string")
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if result["type"] != "object" {
		t.Errorf("expected type to be 'object', got '%v'", result["type"])
	}
}
