package observability

import (
	"errors"
	"testing"
	"time"
)

func TestAttribute_String(t *testing.T) {
	attr := String("key", "value")
	if attr.Key != "key" {
		t.Errorf("Expected key 'key', got '%s'", attr.Key)
	}
	if attr.Value != "value" {
		t.Errorf("Expected value 'value', got '%v'", attr.Value)
	}
}

func TestAttribute_Int(t *testing.T) {
	attr := Int("count", 42)
	if attr.Key != "count" {
		t.Errorf("Expected key 'count', got '%s'", attr.Key)
	}
	if attr.Value != 42 {
		t.Errorf("Expected value 42, got '%v'", attr.Value)
	}
}

func TestAttribute_Int64(t *testing.T) {
	attr := Int64("count", 9223372036854775807)
	if attr.Key != "count" {
		t.Errorf("Expected key 'count', got '%s'", attr.Key)
	}
	if attr.Value != int64(9223372036854775807) {
		t.Errorf("Expected value 9223372036854775807, got '%v'", attr.Value)
	}
}

func TestAttribute_Float64(t *testing.T) {
	attr := Float64("value", 3.14159)
	if attr.Key != "value" {
		t.Errorf("Expected key 'value', got '%s'", attr.Key)
	}
	if attr.Value != 3.14159 {
		t.Errorf("Expected value 3.14159, got '%v'", attr.Value)
	}
}

func TestAttribute_Bool(t *testing.T) {
	attr := Bool("flag", true)
	if attr.Key != "flag" {
		t.Errorf("Expected key 'flag', got '%s'", attr.Key)
	}
	if attr.Value != true {
		t.Errorf("Expected value true, got '%v'", attr.Value)
	}

	attr2 := Bool("flag", false)
	if attr2.Value != false {
		t.Errorf("Expected value false, got '%v'", attr2.Value)
	}
}

func TestAttribute_Duration(t *testing.T) {
	duration := 5 * time.Second
	attr := Duration("latency", duration)
	if attr.Key != "latency" {
		t.Errorf("Expected key 'latency', got '%s'", attr.Key)
	}
	if attr.Value != duration {
		t.Errorf("Expected value %v, got '%v'", duration, attr.Value)
	}
}

func TestAttribute_Error(t *testing.T) {
	testErr := errors.New("test error")
	attr := Error(testErr)
	if attr.Key != "error" {
		t.Errorf("Expected key 'error', got '%s'", attr.Key)
	}
	if attr.Value != "test error" {
		t.Errorf("Expected value 'test error', got '%v'", attr.Value)
	}
}

func TestAttribute_Error_Nil(t *testing.T) {
	attr := Error(nil)
	if attr.Key != "error" {
		t.Errorf("Expected key 'error', got '%s'", attr.Key)
	}
	if attr.Value != "" {
		t.Errorf("Expected empty value for nil error, got '%v'", attr.Value)
	}
}

func TestStatusCode_Values(t *testing.T) {
	if StatusUnset != 0 {
		t.Errorf("Expected StatusUnset to be 0, got %d", StatusUnset)
	}
	if StatusOK != 1 {
		t.Errorf("Expected StatusOK to be 1, got %d", StatusOK)
	}
	if StatusError != 2 {
		t.Errorf("Expected StatusError to be 2, got %d", StatusError)
	}
}

func TestAttribute_MultipleTypes(t *testing.T) {
	attrs := []Attribute{
		String("name", "test"),
		Int("count", 10),
		Int64("bigcount", 100000),
		Float64("rate", 0.95),
		Bool("enabled", true),
		Duration("timeout", 30*time.Second),
		Error(errors.New("sample error")),
	}

	if len(attrs) != 7 {
		t.Errorf("Expected 7 attributes, got %d", len(attrs))
	}

	// Verify each attribute type
	expectedKeys := []string{"name", "count", "bigcount", "rate", "enabled", "timeout", "error"}
	for i, attr := range attrs {
		if attr.Key != expectedKeys[i] {
			t.Errorf("Expected key '%s', got '%s'", expectedKeys[i], attr.Key)
		}
		if attr.Value == nil {
			t.Errorf("Attribute %s has nil value", attr.Key)
		}
	}
}

func TestAttribute_ZeroValues(t *testing.T) {
	tests := []struct {
		name     string
		attr     Attribute
		wantZero bool
	}{
		{"empty string", String("key", ""), true},
		{"zero int", Int("key", 0), true},
		{"zero int64", Int64("key", 0), true},
		{"zero float64", Float64("key", 0.0), true},
		{"false bool", Bool("key", false), true},
		{"zero duration", Duration("key", 0), true},
		{"non-zero string", String("key", "value"), false},
		{"non-zero int", Int("key", 1), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.attr.Key != "key" {
				t.Errorf("Expected key 'key', got '%s'", tt.attr.Key)
			}
			// All values should be set, even if zero
			if tt.attr.Value == nil {
				t.Error("Value should not be nil")
			}
		})
	}
}

func BenchmarkAttribute_String(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = String("key", "value")
	}
}

func BenchmarkAttribute_Int(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Int("key", 42)
	}
}

func BenchmarkAttribute_Float64(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Float64("key", 3.14)
	}
}

func BenchmarkAttribute_Bool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Bool("key", true)
	}
}

func BenchmarkAttribute_Duration(b *testing.B) {
	d := 5 * time.Second
	for i := 0; i < b.N; i++ {
		_ = Duration("key", d)
	}
}

func BenchmarkAttribute_Error(b *testing.B) {
	err := errors.New("test error")
	for i := 0; i < b.N; i++ {
		_ = Error(err)
	}
}
