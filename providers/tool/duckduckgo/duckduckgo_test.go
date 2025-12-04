package duckduckgo

import (
	"testing"
)

// TestToolCreation tests that the tools are created correctly (unit test - no external calls)
func TestToolCreation(t *testing.T) {
	t.Run("Base tool", func(t *testing.T) {
		tool := NewDuckDuckGoSearchTool()
		if tool.Name != "DuckDuckGoSearch" {
			t.Errorf("Tool name = %v, want DuckDuckGoSearch", tool.Name)
		}
		if tool.Description == "" {
			t.Error("Tool description is empty")
		}
		if tool.Function == nil {
			t.Error("Tool function is nil")
		}
	})

	t.Run("Advanced tool", func(t *testing.T) {
		tool := NewDuckDuckGoSearchAdvancedTool()
		if tool.Name != "DuckDuckGoSearchAdvanced" {
			t.Errorf("Tool name = %v, want DuckDuckGoSearchAdvanced", tool.Name)
		}
		if tool.Description == "" {
			t.Error("Tool description is empty")
		}
		if tool.Function == nil {
			t.Error("Tool function is nil")
		}
	})
}

// TestInputStructure tests the Input struct fields (unit test - no external calls)
func TestInputStructure(t *testing.T) {
	input := Input{
		Query: "test query",
	}

	if input.Query != "test query" {
		t.Errorf("Input.Query = %v, want 'test query'", input.Query)
	}
}

// TestOutputStructure tests the Output struct fields (unit test - no external calls)
func TestOutputStructure(t *testing.T) {
	output := Output{
		Query:   "test",
		Summary: "test summary",
	}

	if output.Query != "test" {
		t.Errorf("Output.Query = %v, want 'test'", output.Query)
	}
	if output.Summary != "test summary" {
		t.Errorf("Output.Summary = %v, want 'test summary'", output.Summary)
	}
}

// TestAdvancedOutputStructure tests the AdvancedOutput struct fields (unit test - no external calls)
func TestAdvancedOutputStructure(t *testing.T) {
	output := AdvancedOutput{
		Query:    "test",
		Type:     "A",
		Abstract: "test abstract",
	}

	if output.Query != "test" {
		t.Errorf("AdvancedOutput.Query = %v, want 'test'", output.Query)
	}
	if output.Type != "A" {
		t.Errorf("AdvancedOutput.Type = %v, want 'A'", output.Type)
	}
	if output.Abstract != "test abstract" {
		t.Errorf("AdvancedOutput.Abstract = %v, want 'test abstract'", output.Abstract)
	}
}
