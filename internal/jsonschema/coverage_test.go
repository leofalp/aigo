package jsonschema

import (
	"reflect"
	"testing"
)

func TestHandleStructType_Coverage(t *testing.T) {
	// We need a nested struct that hits all branches in handleStructType
	type Nested struct {
		Exported   string
		unexported string
		Ignored    string `json:"-"`
		Named      string `json:"custom_name"`
		OmitEmpty  string `json:"omit_name,omitempty"`
	}

	type Root struct {
		N1 Nested
		N2 Nested // Hits the visited cache
	}

	schema := GenerateJSONSchema[Root]()
	if schema.Type != "object" {
		t.Errorf("Expected object")
	}

	// Check N1
	n1 := schema.Properties["N1"]
	if n1 == nil {
		t.Fatalf("N1 missing")
	}
	if _, ok := n1.Properties["Exported"]; !ok {
		t.Errorf("Exported missing")
	}
	if _, ok := n1.Properties["unexported"]; ok {
		t.Errorf("unexported should be missing")
	}
	if _, ok := n1.Properties["Ignored"]; ok {
		t.Errorf("Ignored should be missing")
	}
	if _, ok := n1.Properties["custom_name"]; !ok {
		t.Errorf("custom_name missing")
	}
	if _, ok := n1.Properties["omit_name"]; !ok {
		t.Errorf("omit_name missing")
	}

	// Check N2 (should be identical to N1, but we just want to ensure it didn't panic and used cache)
	n2 := schema.Properties["N2"]
	if n2 == nil {
		t.Fatalf("N2 missing")
	}
}

func TestHandleStructType_Recursive_Coverage(t *testing.T) {
	// We need a recursive nested struct to hit the hasRecursion == true branch in handleStructType
	type RecursiveNested struct {
		Exported   string
		unexported string
		Ignored    string `json:"-"`
		Named      string `json:"custom_name"`
		OmitEmpty  string `json:"omit_name,omitempty"`
		Self       *RecursiveNested
	}

	type Root struct {
		N1 RecursiveNested
		N2 RecursiveNested // Hits the visited cache
	}

	schema := GenerateJSONSchema[Root]()
	if schema.Type != "object" {
		t.Errorf("Expected object")
	}

	// N1 should be a reference
	n1 := schema.Properties["N1"]
	if n1 == nil {
		t.Fatalf("N1 missing")
	}
	if n1.Ref == "" {
		t.Errorf("Expected N1 to be a reference")
	}
}

func TestCheckRecursion_Visited(t *testing.T) {
	// To hit visited[currentType] == true
	type Shared struct {
		Value string
	}
	type Root struct {
		S1 Shared
		S2 Shared
	}

	schema := GenerateJSONSchema[Root]()
	if schema.Type != "object" {
		t.Errorf("Expected object")
	}
}

func TestGenerateDefName_Anonymous(t *testing.T) {
	// To hit the anonymous struct branch in generateDefName
	type Root struct {
		Anon struct {
			Value string
		}
	}

	schema := GenerateJSONSchema[Root]()
	if schema.Type != "object" {
		t.Errorf("Expected object")
	}
}

func TestGenerateDefName_AnonymousRoot(t *testing.T) {
	// To hit the anonymous struct branch in generateDefName
	schema := GenerateJSONSchema[struct{ Value string }]()
	if schema.Type != "object" {
		t.Errorf("Expected object")
	}
}

func TestCheckRecursion_DirectCalls(t *testing.T) {
	// We can call checkRecursion directly to hit the dead code branches
	type Target struct {
		Value string
	}

	targetType := reflect.TypeFor[Target]()

	// Test reflect.Slice branch
	sliceType := reflect.TypeFor[[]Target]()
	if !checkRecursion(targetType, sliceType, make(map[reflect.Type]bool)) {
		t.Errorf("Expected true for slice of Target")
	}

	// Test reflect.Slice branch with pointer
	slicePtrType := reflect.TypeFor[[]*Target]()
	if !checkRecursion(targetType, slicePtrType, make(map[reflect.Type]bool)) {
		t.Errorf("Expected true for slice of pointer to Target")
	}

	// Test reflect.Slice branch with nested struct
	type Nested struct {
		T Target
	}
	sliceNestedType := reflect.TypeFor[[]Nested]()
	if !checkRecursion(targetType, sliceNestedType, make(map[reflect.Type]bool)) {
		t.Errorf("Expected true for slice of Nested")
	}

	// Test reflect.Array branch
	arrayType := reflect.TypeFor[[5]Target]()
	if !checkRecursion(targetType, arrayType, make(map[reflect.Type]bool)) {
		t.Errorf("Expected true for array of Target")
	}

	// Test reflect.Ptr branch
	ptrType := reflect.TypeFor[*Target]()
	if !checkRecursion(targetType, ptrType, make(map[reflect.Type]bool)) {
		t.Errorf("Expected true for pointer to Target")
	}

	// Test reflect.Ptr branch with nested struct
	ptrNestedType := reflect.TypeFor[*Nested]()
	if !checkRecursion(targetType, ptrNestedType, make(map[reflect.Type]bool)) {
		t.Errorf("Expected true for pointer to Nested")
	}
}

func TestCheckRecursion_DirectCalls_False(t *testing.T) {
	type Target struct {
		Value string
	}
	type Other struct {
		Value string
	}

	targetType := reflect.TypeFor[Target]()

	// Test reflect.Slice branch false
	sliceOtherType := reflect.TypeFor[[]Other]()
	if checkRecursion(targetType, sliceOtherType, make(map[reflect.Type]bool)) {
		t.Errorf("Expected false for slice of Other")
	}

	// Test reflect.Ptr branch false
	ptrOtherType := reflect.TypeFor[*Other]()
	if checkRecursion(targetType, ptrOtherType, make(map[reflect.Type]bool)) {
		t.Errorf("Expected false for pointer to Other")
	}

	// Test default branch false
	intType := reflect.TypeFor[int]()
	if checkRecursion(targetType, intType, make(map[reflect.Type]bool)) {
		t.Errorf("Expected false for int")
	}
}

func TestCheckRecursion_MutualRecursion(t *testing.T) {
	type B struct {
		A *struct {
			B *B
		}
	}
	type A struct {
		B *B
	}

	schema := GenerateJSONSchema[A]()
	if schema.Type != "object" {
		t.Errorf("Expected object")
	}
}
