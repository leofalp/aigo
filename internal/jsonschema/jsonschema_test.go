package jsonschema

import (
	"testing"
)

func TestGeneratesStringSchema(t *testing.T) {
	schema, err := GenerateJSONSchema[string]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "string" {
		t.Errorf("Expected type 'string', got '%s'", schema.Type)
	}
}

func TestGeneratesIntegerSchema(t *testing.T) {
	schema, err := GenerateJSONSchema[int]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "integer" {
		t.Errorf("Expected type 'integer', got '%s'", schema.Type)
	}
}

func TestGeneratesNumberSchemaForFloat(t *testing.T) {
	schema, err := GenerateJSONSchema[float32]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "number" {
		t.Errorf("Expected type 'number', got '%s'", schema.Type)
	}
}

func TestGeneratesBooleanSchema(t *testing.T) {
	schema, err := GenerateJSONSchema[bool]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "boolean" {
		t.Errorf("Expected type 'boolean', got '%s'", schema.Type)
	}
}

func TestGeneratesArraySchemaForSlice(t *testing.T) {
	schema, err := GenerateJSONSchema[[]string]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "array" {
		t.Errorf("Expected type 'array', got '%s'", schema.Type)
	}
	if schema.Items == nil {
		t.Error("Expected items to be defined")
	}
	if schema.Items.Type != "string" {
		t.Errorf("Expected items type 'string', got '%s'", schema.Items.Type)
	}
}

func TestGeneratesArraySchemaForIntSlice(t *testing.T) {
	schema, err := GenerateJSONSchema[[]int]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "array" {
		t.Errorf("Expected type 'array', got '%s'", schema.Type)
	}
	if schema.Items == nil {
		t.Error("Expected items to be defined")
	}
	if schema.Items.Type != "integer" {
		t.Errorf("Expected items type 'integer', got '%s'", schema.Items.Type)
	}
}

func TestGeneratesObjectSchemaForMap(t *testing.T) {
	schema, err := GenerateJSONSchema[map[string]string]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if schema.AdditionalProperties == nil {
		t.Error("Expected additionalProperties to be defined")
	}
}

func TestGeneratesObjectSchemaForMapWithIntValues(t *testing.T) {
	schema, err := GenerateJSONSchema[map[string]int]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if schema.AdditionalProperties == nil {
		t.Error("Expected additionalProperties to be defined")
	}
	valueSchema, ok := schema.AdditionalProperties.(*Schema)
	if !ok {
		t.Error("Expected additionalProperties to be a Schema")
	}
	if valueSchema.Type != "integer" {
		t.Errorf("Expected additionalProperties type 'integer', got '%s'", valueSchema.Type)
	}
}

func TestGeneratesSchemaForSimpleStruct(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}

	schema, err := GenerateJSONSchema[Person]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if schema.Properties == nil {
		t.Fatal("Expected properties to be defined")
	}
	if _, exists := schema.Properties["Name"]; !exists {
		t.Error("Expected 'Name' property to exist")
	}
	if _, exists := schema.Properties["Age"]; !exists {
		t.Error("Expected 'Age' property to exist")
	}
	if schema.Properties["Name"].Type != "string" {
		t.Errorf("Expected Name type 'string', got '%s'", schema.Properties["Name"].Type)
	}
	if schema.Properties["Age"].Type != "integer" {
		t.Errorf("Expected Age type 'integer', got '%s'", schema.Properties["Age"].Type)
	}
}

func TestHandlesJSONTags(t *testing.T) {
	type User struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}

	schema, err := GenerateJSONSchema[User]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if _, exists := schema.Properties["first_name"]; !exists {
		t.Error("Expected 'first_name' property to exist")
	}
	if _, exists := schema.Properties["last_name"]; !exists {
		t.Error("Expected 'last_name' property to exist")
	}
	if _, exists := schema.Properties["FirstName"]; exists {
		t.Error("Did not expect 'FirstName' property to exist")
	}
}

func TestIgnoresFieldsWithJSONDashTag(t *testing.T) {
	type Data struct {
		Public  string
		Private string `json:"-"`
	}

	schema, err := GenerateJSONSchema[Data]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if _, exists := schema.Properties["Public"]; !exists {
		t.Error("Expected 'Public' property to exist")
	}
	if _, exists := schema.Properties["Private"]; exists {
		t.Error("Did not expect 'Private' property to exist")
	}
}

func TestIgnoresUnexportedFields(t *testing.T) {
	type Data struct {
		Public  string
		private string
	}

	schema, err := GenerateJSONSchema[Data]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if _, exists := schema.Properties["Public"]; !exists {
		t.Error("Expected 'Public' property to exist")
	}
	if _, exists := schema.Properties["private"]; exists {
		t.Error("Did not expect 'private' property to exist")
	}
}

func TestMarksFieldsAsRequiredWhenNotPointerAndNoOmitempty(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}
	// TODO Per default vogliamo che siano required o no?

	schema, err := GenerateJSONSchema[Person]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(schema.Required) != 2 {
		t.Errorf("Expected 2 required fields, got %d", len(schema.Required))
	}
	hasName := false
	hasAge := false
	for _, field := range schema.Required {
		if field == "Name" {
			hasName = true
		}
		if field == "Age" {
			hasAge = true
		}
	}
	if !hasName {
		t.Error("Expected 'Name' to be required")
	}
	if !hasAge {
		t.Error("Expected 'Age' to be required")
	}
}

func TestDoesNotMarkFieldsAsRequiredWhenOmitempty(t *testing.T) {
	type Person struct {
		Name string `json:"name,omitempty"`
		Age  int    `json:"age,omitempty"`
	}

	schema, err := GenerateJSONSchema[Person]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(schema.Required) != 0 {
		t.Errorf("Expected 0 required fields, got %d", len(schema.Required))
	}
}

func TestDoesNotMarkPointerFieldsAsRequired(t *testing.T) {
	type Person struct {
		Name *string
		Age  *int
	}

	schema, err := GenerateJSONSchema[Person]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(schema.Required) != 0 {
		t.Errorf("Expected 0 required fields, got %d", len(schema.Required))
	}
}

func TestHandlesPointerFields(t *testing.T) {
	type Person struct {
		Name *string
	}

	schema, err := GenerateJSONSchema[Person]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Properties["Name"].Type != "string" {
		t.Errorf("Expected Name type 'string', got '%s'", schema.Properties["Name"].Type)
	}
}

func TestHandlesNestedStructs(t *testing.T) {
	type Address struct {
		Street string
		City   string
	}
	type Person struct {
		Name    string
		Address Address
	}

	schema, err := GenerateJSONSchema[Person]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if _, exists := schema.Properties["Address"]; !exists {
		t.Fatal("Expected 'Address' property to exist")
	}
	addressSchema := schema.Properties["Address"]
	if addressSchema.Type != "object" {
		t.Errorf("Expected Address type 'object', got '%s'", addressSchema.Type)
	}
	if _, exists := addressSchema.Properties["Street"]; !exists {
		t.Error("Expected 'Street' property in Address")
	}
	if _, exists := addressSchema.Properties["City"]; !exists {
		t.Error("Expected 'City' property in Address")
	}
}

func TestHandlesSliceOfStructs(t *testing.T) {
	type Item struct {
		Name  string
		Price int
	}

	schema, err := GenerateJSONSchema[[]Item]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "array" {
		t.Errorf("Expected type 'array', got '%s'", schema.Type)
	}
	if schema.Items == nil {
		t.Fatal("Expected items to be defined")
	}
	if schema.Items.Type != "object" {
		t.Errorf("Expected items type 'object', got '%s'", schema.Items.Type)
	}
	if _, exists := schema.Items.Properties["Name"]; !exists {
		t.Error("Expected 'Name' property in items")
	}
	if _, exists := schema.Items.Properties["Price"]; !exists {
		t.Error("Expected 'Price' property in items")
	}
}

func TestHandlesMapWithStructValues(t *testing.T) {
	type Item struct {
		Name  string
		Price int
	}

	schema, err := GenerateJSONSchema[map[string]Item]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if schema.AdditionalProperties == nil {
		t.Fatal("Expected additionalProperties to be defined")
	}
	valueSchema, ok := schema.AdditionalProperties.(*Schema)
	if !ok {
		t.Fatal("Expected additionalProperties to be a Schema")
	}
	if valueSchema.Type != "object" {
		t.Errorf("Expected additionalProperties type 'object', got '%s'", valueSchema.Type)
	}
}

func TestHandlesDescriptionTag(t *testing.T) {
	type User struct {
		Name string `json:"name" jsonschema:"description=The user's full name"`
	}

	schema, err := GenerateJSONSchema[User]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Properties["name"].Description != "The user's full name" {
		t.Errorf("Expected description 'The user's full name', got '%s'", schema.Properties["name"].Description)
	}
}

func TestHandlesRequiredTag(t *testing.T) {
	type User struct {
		Name string `json:"name,omitempty" jsonschema:"required"`
		Age  int    `json:"age,omitempty"`
	}

	schema, err := GenerateJSONSchema[User]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(schema.Required) != 1 {
		t.Errorf("Expected 1 required field, got %d", len(schema.Required))
	}
	if len(schema.Required) > 0 && schema.Required[0] != "name" {
		t.Errorf("Expected 'name' to be required, got '%s'", schema.Required[0])
	}
}

func TestHandlesEnumTagForString(t *testing.T) {
	type Status struct {
		Value string `json:"value" jsonschema:"enum=active,enum=inactive,enum=pending"`
	}

	schema, err := GenerateJSONSchema[Status]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(schema.Properties["value"].Enum) != 3 {
		t.Errorf("Expected 3 enum values, got %d", len(schema.Properties["value"].Enum))
	}
	expectedEnums := []string{"active", "inactive", "pending"}
	for i, expected := range expectedEnums {
		if schema.Properties["value"].Enum[i] != expected {
			t.Errorf("Expected enum[%d] to be '%s', got '%v'", i, expected, schema.Properties["value"].Enum[i])
		}
	}
}

func TestHandlesEnumTagForInteger(t *testing.T) {
	type Priority struct {
		Level int `json:"level" jsonschema:"enum=1,enum=2,enum=3"`
	}

	schema, err := GenerateJSONSchema[Priority]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(schema.Properties["level"].Enum) != 3 {
		t.Errorf("Expected 3 enum values, got %d", len(schema.Properties["level"].Enum))
	}
	expectedEnums := []int64{1, 2, 3}
	for i, expected := range expectedEnums {
		if schema.Properties["level"].Enum[i] != expected {
			t.Errorf("Expected enum[%d] to be %d, got %v", i, expected, schema.Properties["level"].Enum[i])
		}
	}
}

func TestHandlesEnumTagForFloat(t *testing.T) {
	type Rating struct {
		Score float64 `json:"score" jsonschema:"enum=1.5,enum=2.5,enum=3.5"`
	}

	schema, err := GenerateJSONSchema[Rating]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(schema.Properties["score"].Enum) != 3 {
		t.Errorf("Expected 3 enum values, got %d", len(schema.Properties["score"].Enum))
	}
	expectedEnums := []float64{1.5, 2.5, 3.5}
	for i, expected := range expectedEnums {
		if schema.Properties["score"].Enum[i] != expected {
			t.Errorf("Expected enum[%d] to be %f, got %v", i, expected, schema.Properties["score"].Enum[i])
		}
	}
}

func TestHandlesEnumTagForBoolean(t *testing.T) {
	type Flag struct {
		Enabled bool `json:"enabled" jsonschema:"enum=true,enum=false"`
	}

	schema, err := GenerateJSONSchema[Flag]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(schema.Properties["enabled"].Enum) != 2 {
		t.Errorf("Expected 2 enum values, got %d", len(schema.Properties["enabled"].Enum))
	}
}

func TestHandlesMultipleJSONSchemaTags(t *testing.T) {
	type User struct {
		Status string `json:"status" jsonschema:"description=User status,enum=active,enum=inactive,required"`
	}

	schema, err := GenerateJSONSchema[User]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Properties["status"].Description != "User status" {
		t.Errorf("Expected description 'User status', got '%s'", schema.Properties["status"].Description)
	}
	if len(schema.Properties["status"].Enum) != 2 {
		t.Errorf("Expected 2 enum values, got %d", len(schema.Properties["status"].Enum))
	}
	if len(schema.Required) != 1 {
		t.Errorf("Expected 1 required field, got %d", len(schema.Required))
	}
}

func TestHandlesRecursiveStructWithSelfReference(t *testing.T) {
	type Node struct {
		Value    string
		Children []*Node
	}

	schema, err := GenerateJSONSchema[Node]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if schema.Defs == nil {
		t.Fatal("Expected $defs to be defined for recursive structure")
	}
	if _, exists := schema.Defs["node"]; !exists {
		t.Error("Expected 'node' definition to exist")
	}
	if schema.Properties["Children"] == nil {
		t.Fatal("Expected 'Children' property to exist")
	}
	if schema.Properties["Children"].Type != "array" {
		t.Errorf("Expected Children type 'array', got '%s'", schema.Properties["Children"].Type)
	}
	if schema.Properties["Children"].Items == nil {
		t.Fatal("Expected Children items to be defined")
	}
	if schema.Properties["Children"].Items.Ref != "#/$defs/node" {
		t.Errorf("Expected Children items to reference '#/$defs/node', got '%s'", schema.Properties["Children"].Items.Ref)
	}
}

func TestHandlesRecursiveStructWithDirectReference(t *testing.T) {
	type LinkedNode struct {
		Value string
		Next  *LinkedNode
	}

	schema, err := GenerateJSONSchema[LinkedNode]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if schema.Defs == nil {
		t.Fatal("Expected $defs to be defined for recursive structure")
	}
}

func TestHandlesMutuallyRecursiveStructs(t *testing.T) {
	type A struct {
		Name string
		B    interface{}
	}

	schema, err := GenerateJSONSchema[A]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if _, exists := schema.Properties["B"]; !exists {
		t.Error("Expected 'B' property to exist")
	}
}

func TestHandlesComplexNestedStructures(t *testing.T) {
	type Metadata struct {
		Tags   []string
		Values map[string]int
	}
	type Item struct {
		ID       int
		Name     string
		Meta     Metadata
		Children []Item
	}

	schema, err := GenerateJSONSchema[Item]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if _, exists := schema.Properties["Meta"]; !exists {
		t.Fatal("Expected 'Meta' property to exist")
	}
	metaSchema := schema.Properties["Meta"]
	if metaSchema.Type != "object" {
		t.Errorf("Expected Meta type 'object', got '%s'", metaSchema.Type)
	}
	if _, exists := metaSchema.Properties["Tags"]; !exists {
		t.Error("Expected 'Tags' property in Meta")
	}
	if _, exists := metaSchema.Properties["Values"]; !exists {
		t.Error("Expected 'Values' property in Meta")
	}
}

func TestHandlesAllIntegerTypes(t *testing.T) {
	types := []struct {
		name  string
		value interface{}
	}{
		{"int", int(0)},
		{"int8", int8(0)},
		{"int16", int16(0)},
		{"int32", int32(0)},
		{"int64", int64(0)},
		{"uint", uint(0)},
		{"uint8", uint8(0)},
		{"uint16", uint16(0)},
		{"uint32", uint32(0)},
		{"uint64", uint64(0)},
	}

	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			var schema *Schema
			var err error
			switch tt.name {
			case "int":
				schema, err = GenerateJSONSchema[int]()
			case "int8":
				schema, err = GenerateJSONSchema[int8]()
			case "int16":
				schema, err = GenerateJSONSchema[int16]()
			case "int32":
				schema, err = GenerateJSONSchema[int32]()
			case "int64":
				schema, err = GenerateJSONSchema[int64]()
			case "uint":
				schema, err = GenerateJSONSchema[uint]()
			case "uint8":
				schema, err = GenerateJSONSchema[uint8]()
			case "uint16":
				schema, err = GenerateJSONSchema[uint16]()
			case "uint32":
				schema, err = GenerateJSONSchema[uint32]()
			case "uint64":
				schema, err = GenerateJSONSchema[uint64]()
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if schema.Type != "integer" {
				t.Errorf("Expected type 'integer' for %s, got '%s'", tt.name, schema.Type)
			}
		})
	}
}

func TestHandlesAllFloatTypes(t *testing.T) {
	types := []struct {
		name  string
		value interface{}
	}{
		{"float32", float32(0)},
		{"float64", float64(0)},
	}

	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			var schema *Schema
			var err error
			switch tt.name {
			case "float32":
				schema, err = GenerateJSONSchema[float32]()
			case "float64":
				schema, err = GenerateJSONSchema[float64]()
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if schema.Type != "number" {
				t.Errorf("Expected type 'number' for %s, got '%s'", tt.name, schema.Type)
			}
		})
	}
}

func TestHandlesEmptyStruct(t *testing.T) {
	type Empty struct{}

	schema, err := GenerateJSONSchema[Empty]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if len(schema.Properties) != 0 {
		t.Errorf("Expected 0 properties, got %d", len(schema.Properties))
	}
}

func TestHandlesStructWithOnlyUnexportedFields(t *testing.T) {
	type Private struct {
		name string
		age  int
	}

	schema, err := GenerateJSONSchema[Private]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if len(schema.Properties) != 0 {
		t.Errorf("Expected 0 properties, got %d", len(schema.Properties))
	}
}

func TestHandlesPointerToStruct(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}

	schema, err := GenerateJSONSchema[*Person]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if _, exists := schema.Properties["Name"]; !exists {
		t.Error("Expected 'Name' property to exist")
	}
}

func TestHandlesNestedPointers(t *testing.T) {
	type Address struct {
		Street string
	}
	type Person struct {
		Name    string
		Address *Address
	}

	schema, err := GenerateJSONSchema[Person]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if _, exists := schema.Properties["Address"]; !exists {
		t.Fatal("Expected 'Address' property to exist")
	}
	addressSchema := schema.Properties["Address"]
	if addressSchema.Type != "object" {
		t.Errorf("Expected Address type 'object', got '%s'", addressSchema.Type)
	}
}

func TestHandlesArrayOfPrimitives(t *testing.T) {
	schema, err := GenerateJSONSchema[[5]int]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "array" {
		t.Errorf("Expected type 'array', got '%s'", schema.Type)
	}
	if schema.Items == nil {
		t.Fatal("Expected items to be defined")
	}
	if schema.Items.Type != "integer" {
		t.Errorf("Expected items type 'integer', got '%s'", schema.Items.Type)
	}
}

func TestHandlesSliceOfPointers(t *testing.T) {
	schema, err := GenerateJSONSchema[[]*string]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "array" {
		t.Errorf("Expected type 'array', got '%s'", schema.Type)
	}
	if schema.Items == nil {
		t.Fatal("Expected items to be defined")
	}
	if schema.Items.Type != "string" {
		t.Errorf("Expected items type 'string', got '%s'", schema.Items.Type)
	}
}

func TestHandlesMapWithComplexKeys(t *testing.T) {
	schema, err := GenerateJSONSchema[map[string][]int]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if schema.AdditionalProperties == nil {
		t.Fatal("Expected additionalProperties to be defined")
	}
	valueSchema, ok := schema.AdditionalProperties.(*Schema)
	if !ok {
		t.Fatal("Expected additionalProperties to be a Schema")
	}
	if valueSchema.Type != "array" {
		t.Errorf("Expected additionalProperties type 'array', got '%s'", valueSchema.Type)
	}
}

func TestHandlesStructWithMixedFieldVisibility(t *testing.T) {
	type Mixed struct {
		Public   string
		private  string
		Exported int
		internal int
	}

	schema, err := GenerateJSONSchema[Mixed]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(schema.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(schema.Properties))
	}
	if _, exists := schema.Properties["Public"]; !exists {
		t.Error("Expected 'Public' property to exist")
	}
	if _, exists := schema.Properties["Exported"]; !exists {
		t.Error("Expected 'Exported' property to exist")
	}
}

func TestHandlesStructWithBothJSONAndJSONSchemaTagsCombined(t *testing.T) {
	type Product struct {
		Name  string  `json:"name" jsonschema:"description=Product name,required"`
		Price float64 `json:"price,omitempty" jsonschema:"description=Product price"`
		Type  string  `json:"type" jsonschema:"enum=physical,enum=digital"`
	}

	schema, err := GenerateJSONSchema[Product]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Properties["name"].Description != "Product name" {
		t.Errorf("Expected name description, got '%s'", schema.Properties["name"].Description)
	}
	if schema.Properties["price"].Description != "Product price" {
		t.Errorf("Expected price description, got '%s'", schema.Properties["price"].Description)
	}
	if len(schema.Properties["type"].Enum) != 2 {
		t.Errorf("Expected 2 enum values for type, got %d", len(schema.Properties["type"].Enum))
	}
}

func TestHandlesDeeplyNestedStructures(t *testing.T) {
	type Level3 struct {
		Value string
	}
	type Level2 struct {
		L3 Level3
	}
	type Level1 struct {
		L2 Level2
	}

	schema, err := GenerateJSONSchema[Level1]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if _, exists := schema.Properties["L2"]; !exists {
		t.Fatal("Expected 'L2' property to exist")
	}
	l2Schema := schema.Properties["L2"]
	if _, exists := l2Schema.Properties["L3"]; !exists {
		t.Fatal("Expected 'L3' property to exist")
	}
	l3Schema := l2Schema.Properties["L3"]
	if _, exists := l3Schema.Properties["Value"]; !exists {
		t.Error("Expected 'Value' property to exist")
	}
}

func TestHandlesSliceOfSlices(t *testing.T) {
	schema, err := GenerateJSONSchema[[][]string]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "array" {
		t.Errorf("Expected type 'array', got '%s'", schema.Type)
	}
	if schema.Items == nil {
		t.Fatal("Expected items to be defined")
	}
	if schema.Items.Type != "array" {
		t.Errorf("Expected items type 'array', got '%s'", schema.Items.Type)
	}
	if schema.Items.Items == nil {
		t.Fatal("Expected nested items to be defined")
	}
	if schema.Items.Items.Type != "string" {
		t.Errorf("Expected nested items type 'string', got '%s'", schema.Items.Items.Type)
	}
}

func TestHandlesMapOfMaps(t *testing.T) {
	schema, err := GenerateJSONSchema[map[string]map[string]int]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	valueSchema, ok := schema.AdditionalProperties.(*Schema)
	if !ok {
		t.Fatal("Expected additionalProperties to be a Schema")
	}
	if valueSchema.Type != "object" {
		t.Errorf("Expected nested map type 'object', got '%s'", valueSchema.Type)
	}
	nestedValueSchema, ok := valueSchema.AdditionalProperties.(*Schema)
	if !ok {
		t.Fatal("Expected nested additionalProperties to be a Schema")
	}
	if nestedValueSchema.Type != "integer" {
		t.Errorf("Expected nested value type 'integer', got '%s'", nestedValueSchema.Type)
	}
}

func TestHandlesStructWithArrayAndMapFields(t *testing.T) {
	type Complex struct {
		Items   []string
		Mapping map[string]int
		Single  string
	}

	schema, err := GenerateJSONSchema[Complex]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Properties["Items"].Type != "array" {
		t.Errorf("Expected Items type 'array', got '%s'", schema.Properties["Items"].Type)
	}
	if schema.Properties["Mapping"].Type != "object" {
		t.Errorf("Expected Mapping type 'object', got '%s'", schema.Properties["Mapping"].Type)
	}
	if schema.Properties["Single"].Type != "string" {
		t.Errorf("Expected Single type 'string', got '%s'", schema.Properties["Single"].Type)
	}
}

func TestGeneratesUniqueDefinitionNames(t *testing.T) {
	type Node struct {
		Value string
		Left  *Node
		Right *Node
	}

	schema, err := GenerateJSONSchema[Node]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Defs == nil {
		t.Fatal("Expected $defs to be defined")
	}
	if len(schema.Defs) == 0 {
		t.Error("Expected at least one definition")
	}
	if _, exists := schema.Defs["node"]; !exists {
		t.Error("Expected 'node' definition to exist")
	}
}

func TestHandlesTreeStructureWithMultipleReferences(t *testing.T) {
	type TreeNode struct {
		Value    int
		Left     *TreeNode
		Right    *TreeNode
		Children []*TreeNode
	}

	schema, err := GenerateJSONSchema[TreeNode]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", schema.Type)
	}
	if schema.Defs == nil {
		t.Fatal("Expected $defs to be defined for recursive structure")
	}
	if schema.Properties["Left"] == nil {
		t.Error("Expected 'Left' property to exist")
	}
	if schema.Properties["Right"] == nil {
		t.Error("Expected 'Right' property to exist")
	}
	if schema.Properties["Children"] == nil {
		t.Error("Expected 'Children' property to exist")
	}
}

func TestHandlesStructWithAllOptionalFields(t *testing.T) {
	type Optional struct {
		Field1 *string `json:"field1,omitempty"`
		Field2 *int    `json:"field2,omitempty"`
		Field3 *bool   `json:"field3,omitempty"`
	}

	schema, err := GenerateJSONSchema[Optional]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(schema.Required) != 0 {
		t.Errorf("Expected 0 required fields, got %d", len(schema.Required))
	}
}

func TestHandlesStructWithMixedRequiredAndOptional(t *testing.T) {
	type Mixed struct {
		Required1 string `json:"required1"`
		Optional1 string `json:"optional1,omitempty"`
		Required2 int    `json:"required2"`
		Optional2 *int   `json:"optional2"`
	}

	schema, err := GenerateJSONSchema[Mixed]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(schema.Required) != 2 {
		t.Errorf("Expected 2 required fields, got %d", len(schema.Required))
	}
	hasRequired1 := false
	hasRequired2 := false
	for _, field := range schema.Required {
		if field == "required1" {
			hasRequired1 = true
		}
		if field == "required2" {
			hasRequired2 = true
		}
	}
	if !hasRequired1 {
		t.Error("Expected 'required1' to be required")
	}
	if !hasRequired2 {
		t.Error("Expected 'required2' to be required")
	}
}

func TestHandlesJsonSchemaTagWithSpaces(t *testing.T) {
	type Tagged struct {
		Field string `json:"field" jsonschema:"description=A field with spaces in description"`
	}

	schema, err := GenerateJSONSchema[Tagged]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Properties["field"].Description != "A field with spaces in description" {
		t.Errorf("Expected description with spaces, got '%s'", schema.Properties["field"].Description)
	}
}

func TestDoesNotCreateDefinitionsForNonRecursiveStructs(t *testing.T) {
	type Simple struct {
		Name string
		Age  int
	}
	type Container struct {
		Data Simple
	}

	schema, err := GenerateJSONSchema[Container]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Defs != nil && len(schema.Defs) > 0 {
		t.Error("Did not expect $defs for non-recursive structures")
	}
}

func TestHandlesAnonymousStructs(t *testing.T) {
	type Container struct {
		Nested struct {
			Value string
		}
	}

	schema, err := GenerateJSONSchema[Container]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if _, exists := schema.Properties["Nested"]; !exists {
		t.Fatal("Expected 'Nested' property to exist")
	}
	nestedSchema := schema.Properties["Nested"]
	if nestedSchema.Type != "object" {
		t.Errorf("Expected Nested type 'object', got '%s'", nestedSchema.Type)
	}
	if _, exists := nestedSchema.Properties["Value"]; !exists {
		t.Error("Expected 'Value' property in nested anonymous struct")
	}
}

func TestHandlesEnumWithSingleValue(t *testing.T) {
	type Single struct {
		Value string `json:"value" jsonschema:"enum=only"`
	}

	schema, err := GenerateJSONSchema[Single]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(schema.Properties["value"].Enum) != 1 {
		t.Errorf("Expected 1 enum value, got %d", len(schema.Properties["value"].Enum))
	}
	if schema.Properties["value"].Enum[0] != "only" {
		t.Errorf("Expected enum value 'only', got '%v'", schema.Properties["value"].Enum[0])
	}
}

func TestHandlesMultipleFieldsWithDifferentJSONSchemaTags(t *testing.T) {
	type MultiTag struct {
		Field1 string `json:"field1" jsonschema:"description=First field"`
		Field2 int    `json:"field2" jsonschema:"enum=1,enum=2"`
		Field3 string `json:"field3" jsonschema:"required"`
		Field4 bool   `json:"field4,omitempty"`
	}

	schema, err := GenerateJSONSchema[MultiTag]()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if schema.Properties["field1"].Description != "First field" {
		t.Error("Expected field1 to have description")
	}
	if len(schema.Properties["field2"].Enum) != 2 {
		t.Error("Expected field2 to have 2 enum values")
	}
	hasField1 := false
	hasField2 := false
	hasField3 := false
	for _, field := range schema.Required {
		if field == "field1" {
			hasField1 = true
		}
		if field == "field2" {
			hasField2 = true
		}
		if field == "field3" {
			hasField3 = true
		}
	}
	if !hasField1 {
		t.Error("Expected field1 to be required (no pointer, no omitempty)")
	}
	if !hasField2 {
		t.Error("Expected field2 to be required (no pointer, no omitempty)")
	}
	if !hasField3 {
		t.Error("Expected field3 to be required (jsonschema:required tag)")
	}
}
