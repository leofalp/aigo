package utils

import (
	"testing"
)

// TestPtr verifies that Ptr returns a non-nil pointer whose dereferenced value
// equals the original input. Each type is tested individually because Go
// generics do not support table-driven tests across different type parameters.
func TestPtr(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		input := 42
		result := Ptr(input)
		if result == nil {
			t.Fatal("expected non-nil pointer, got nil")
		}
		if *result != input {
			t.Errorf("expected *result=%d, got %d", input, *result)
		}
	})

	t.Run("string", func(t *testing.T) {
		input := "hello"
		result := Ptr(input)
		if result == nil {
			t.Fatal("expected non-nil pointer, got nil")
		}
		if *result != input {
			t.Errorf("expected *result=%q, got %q", input, *result)
		}
	})

	t.Run("bool", func(t *testing.T) {
		input := true
		result := Ptr(input)
		if result == nil {
			t.Fatal("expected non-nil pointer, got nil")
		}
		if *result != input {
			t.Errorf("expected *result=%v, got %v", input, *result)
		}
	})

	t.Run("float64", func(t *testing.T) {
		input := 3.14
		result := Ptr(input)
		if result == nil {
			t.Fatal("expected non-nil pointer, got nil")
		}
		if *result != input {
			t.Errorf("expected *result=%v, got %v", input, *result)
		}
	})
}
