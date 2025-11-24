package parse

import (
	"testing"
)

func TestParseStringAs_String(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "simple string",
			input:   "hello world",
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "string with special characters",
			input:   "hello\nworld\t!",
			want:    "hello\nworld\t!",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[string](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_Bool(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		{
			name:    "true",
			input:   "true",
			want:    true,
			wantErr: false,
		},
		{
			name:    "false",
			input:   "false",
			want:    false,
			wantErr: false,
		},
		{
			name:    "1 as true",
			input:   "1",
			want:    true,
			wantErr: false,
		},
		{
			name:    "0 as false",
			input:   "0",
			want:    false,
			wantErr: false,
		},
		{
			name:    "invalid bool",
			input:   "not a bool",
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[bool](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_Int(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:    "positive int",
			input:   "42",
			want:    42,
			wantErr: false,
		},
		{
			name:    "negative int",
			input:   "-123",
			want:    -123,
			wantErr: false,
		},
		{
			name:    "zero",
			input:   "0",
			want:    0,
			wantErr: false,
		},
		{
			name:    "invalid int",
			input:   "not a number",
			want:    0,
			wantErr: true,
		},
		{
			name:    "float as int should fail",
			input:   "42.5",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[int](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_Float(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{
			name:    "positive float",
			input:   "42.5",
			want:    42.5,
			wantErr: false,
		},
		{
			name:    "negative float",
			input:   "-123.456",
			want:    -123.456,
			wantErr: false,
		},
		{
			name:    "integer as float",
			input:   "42",
			want:    42.0,
			wantErr: false,
		},
		{
			name:    "scientific notation",
			input:   "1.23e10",
			want:    1.23e10,
			wantErr: false,
		},
		{
			name:    "invalid float",
			input:   "not a number",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[float64](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_Uint(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint
		wantErr bool
	}{
		{
			name:    "positive uint",
			input:   "42",
			want:    42,
			wantErr: false,
		},
		{
			name:    "zero",
			input:   "0",
			want:    0,
			wantErr: false,
		},
		{
			name:    "negative should fail",
			input:   "-123",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid uint",
			input:   "not a number",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[uint](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_Struct(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		input   string
		want    Person
		wantErr bool
	}{
		{
			name:    "valid JSON",
			input:   `{"name":"John","age":30}`,
			want:    Person{Name: "John", Age: 30},
			wantErr: false,
		},
		{
			name:    "valid JSON with spaces",
			input:   `{"name": "Jane", "age": 25}`,
			want:    Person{Name: "Jane", Age: 25},
			wantErr: false,
		},
		{
			name:    "missing quotes around keys (should be repaired)",
			input:   `{name: "Alice", age: 28}`,
			want:    Person{Name: "Alice", Age: 28},
			wantErr: false,
		},
		{
			name:    "single quotes (should be repaired)",
			input:   `{'name': 'Bob', 'age': 35}`,
			want:    Person{Name: "Bob", Age: 35},
			wantErr: false,
		},
		{
			name:    "trailing comma (should be repaired)",
			input:   `{"name": "Charlie", "age": 40,}`,
			want:    Person{Name: "Charlie", Age: 40},
			wantErr: false,
		},
		{
			name:    "missing closing bracket (should be repaired)",
			input:   `{"name": "David", "age": 45`,
			want:    Person{Name: "David", Age: 45},
			wantErr: false,
		},
		{
			name:    "completely invalid JSON",
			input:   `this is not json at all`,
			want:    Person{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[Person](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseStringAs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_StructPointer(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		input   string
		want    *Person
		wantErr bool
	}{
		{
			name:    "valid JSON for pointer",
			input:   `{"name":"John","age":30}`,
			want:    &Person{Name: "John", Age: 30},
			wantErr: false,
		},
		{
			name:    "repaired JSON for pointer",
			input:   `{name: 'Alice', age: 28}`,
			want:    &Person{Name: "Alice", Age: 28},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[*Person](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && (got == nil || *got != *tt.want) {
				t.Errorf("ParseStringAs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_Slice(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:    "valid JSON array",
			input:   `["apple","banana","cherry"]`,
			want:    []string{"apple", "banana", "cherry"},
			wantErr: false,
		},
		{
			name:    "valid JSON array with spaces",
			input:   `["apple", "banana", "cherry"]`,
			want:    []string{"apple", "banana", "cherry"},
			wantErr: false,
		},
		{
			name:    "single quotes (should be repaired)",
			input:   `['apple', 'banana', 'cherry']`,
			want:    []string{"apple", "banana", "cherry"},
			wantErr: false,
		},
		{
			name:    "trailing comma (should be repaired)",
			input:   `["apple", "banana", "cherry",]`,
			want:    []string{"apple", "banana", "cherry"},
			wantErr: false,
		},
		{
			name:    "empty array",
			input:   `[]`,
			want:    []string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[[]string](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !stringSlicesEqual(got, tt.want) {
				t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_Map(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:  "valid JSON object",
			input: `{"key1":"value1","key2":"value2"}`,
			want: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name:  "missing quotes (should be repaired)",
			input: `{key1: "value1", key2: "value2"}`,
			want: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name:  "single quotes (should be repaired)",
			input: `{'key1': 'value1', 'key2': 'value2'}`,
			want: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
		{
			name:    "empty object",
			input:   `{}`,
			want:    map[string]interface{}{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[map[string]interface{}](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !mapsEqual(got, tt.want) {
				t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_PythonConstants(t *testing.T) {
	type Config struct {
		Enabled interface{} `json:"enabled"`
		Value   interface{} `json:"value"`
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "Python None (should be repaired to null)",
			input:   `{"enabled": None, "value": 42}`,
			wantErr: false,
		},
		{
			name:    "Python True (should be repaired to true)",
			input:   `{"enabled": True, "value": 42}`,
			wantErr: false,
		},
		{
			name:    "Python False (should be repaired to false)",
			input:   `{"enabled": False, "value": 42}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseStringAs[Config](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseStringAs_CommentsAndCodeBlocks(t *testing.T) {
	type Data struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		input   string
		want    Data
		wantErr bool
	}{
		{
			name: "JSON with single-line comments (should be repaired)",
			input: `{
				// This is a comment
				"name": "John",
				"age": 30
			}`,
			want:    Data{Name: "John", Age: 30},
			wantErr: false,
		},
		{
			name: "JSON with multi-line comments (should be repaired)",
			input: `{
				/* This is a
				   multi-line comment */
				"name": "Jane",
				"age": 25
			}`,
			want:    Data{Name: "Jane", Age: 25},
			wantErr: false,
		},
		{
			name: "JSON in code block (should be repaired)",
			input: "```json\n" +
				`{"name": "Bob", "age": 35}` + "\n" +
				"```",
			want:    Data{Name: "Bob", Age: 35},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[Data](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseStringAs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_TruncatedJSON(t *testing.T) {
	type Person struct {
		Name  string `json:"name"`
		Age   int    `json:"age"`
		Email string `json:"email,omitempty"`
	}

	tests := []struct {
		name    string
		input   string
		want    Person
		wantErr bool
	}{
		{
			name:    "truncated JSON (should be repaired)",
			input:   `{"name": "John", "age": 30`,
			want:    Person{Name: "John", Age: 30},
			wantErr: false,
		},
		{
			name:    "truncated nested JSON (should be repaired)",
			input:   `{"name": "Jane", "age": 25, "email": "jane@ex`,
			want:    Person{Name: "Jane", Age: 25},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[Person](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Name != tt.want.Name && got.Age != tt.want.Age {
				t.Errorf("ParseStringAs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Helper function to compare string slices
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Helper function to compare maps
func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || v != bv {
			return false
		}
	}
	return true
}

func TestParseStringAs_SchemaWrappedValues(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		input   string
		want    Person
		wantErr bool
	}{
		{
			name:    "schema-wrapped struct fields",
			input:   `{"name": {"type": "string", "value": "John"}, "age": {"type": "integer", "value": 30}}`,
			want:    Person{Name: "John", Age: 30},
			wantErr: false,
		},
		{
			name:    "mixed wrapped and unwrapped fields",
			input:   `{"name": {"type": "string", "value": "Alice"}, "age": 25}`,
			want:    Person{Name: "Alice", Age: 25},
			wantErr: false,
		},
		{
			name:    "single wrapped field",
			input:   `{"name": "Bob", "age": {"type": "integer", "value": 35}}`,
			want:    Person{Name: "Bob", Age: 35},
			wantErr: false,
		},
		{
			name:    "schema wrapper with malformed JSON (should repair then unwrap)",
			input:   `{name: {type: "string", value: "Charlie"}, age: {type: "integer", value: 40}}`,
			want:    Person{Name: "Charlie", Age: 40},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[Person](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseStringAs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_SchemaWrappedPrimitives(t *testing.T) {
	t.Run("string primitives", func(t *testing.T) {
		tests := []struct {
			name    string
			input   string
			want    string
			wantErr bool
		}{
			{
				name:    "wrapped string value",
				input:   `{"type": "string", "value": "hello"}`,
				want:    "hello",
				wantErr: false,
			},
			{
				name:    "wrapped with extra whitespace",
				input:   `{ "type": "string", "value": "world" }`,
				want:    "world",
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ParseStringAs[string](tt.input)
				if (err != nil) != tt.wantErr {
					t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !tt.wantErr && got != tt.want {
					t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("int primitives", func(t *testing.T) {
		tests := []struct {
			name    string
			input   string
			want    int
			wantErr bool
		}{
			{
				name:    "wrapped int value",
				input:   `{"type": "integer", "value": 42}`,
				want:    42,
				wantErr: false,
			},
			{
				name:    "wrapped negative int",
				input:   `{"type": "integer", "value": -123}`,
				want:    -123,
				wantErr: false,
			},
			{
				name:    "wrapped zero",
				input:   `{"type": "integer", "value": 0}`,
				want:    0,
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ParseStringAs[int](tt.input)
				if (err != nil) != tt.wantErr {
					t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !tt.wantErr && got != tt.want {
					t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("float primitives", func(t *testing.T) {
		tests := []struct {
			name    string
			input   string
			want    float64
			wantErr bool
		}{
			{
				name:    "wrapped float value",
				input:   `{"type": "number", "value": 3.14}`,
				want:    3.14,
				wantErr: false,
			},
			{
				name:    "wrapped negative float",
				input:   `{"type": "number", "value": -99.99}`,
				want:    -99.99,
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ParseStringAs[float64](tt.input)
				if (err != nil) != tt.wantErr {
					t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !tt.wantErr && got != tt.want {
					t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("bool primitives", func(t *testing.T) {
		tests := []struct {
			name    string
			input   string
			want    bool
			wantErr bool
		}{
			{
				name:    "wrapped true",
				input:   `{"type": "boolean", "value": true}`,
				want:    true,
				wantErr: false,
			},
			{
				name:    "wrapped false",
				input:   `{"type": "boolean", "value": false}`,
				want:    false,
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ParseStringAs[bool](tt.input)
				if (err != nil) != tt.wantErr {
					t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !tt.wantErr && got != tt.want {
					t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("uint primitives", func(t *testing.T) {
		tests := []struct {
			name    string
			input   string
			want    uint
			wantErr bool
		}{
			{
				name:    "wrapped uint value",
				input:   `{"type": "integer", "value": 42}`,
				want:    42,
				wantErr: false,
			},
			{
				name:    "wrapped zero uint",
				input:   `{"type": "integer", "value": 0}`,
				want:    0,
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ParseStringAs[uint](tt.input)
				if (err != nil) != tt.wantErr {
					t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !tt.wantErr && got != tt.want {
					t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
				}
			})
		}
	})
}

func TestParseStringAs_SchemaWrappedArrays(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:    "array with wrapped elements",
			input:   `[{"type": "string", "value": "apple"}, {"type": "string", "value": "banana"}]`,
			want:    []string{"apple", "banana"},
			wantErr: false,
		},
		{
			name:    "array with mixed wrapped and unwrapped",
			input:   `[{"type": "string", "value": "apple"}, "banana", {"type": "string", "value": "cherry"}]`,
			want:    []string{"apple", "banana", "cherry"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[[]string](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !stringSlicesEqual(got, tt.want) {
				t.Errorf("ParseStringAs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_SchemaWrappedNested(t *testing.T) {
	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}

	type Person struct {
		Name    string  `json:"name"`
		Address Address `json:"address"`
	}

	tests := []struct {
		name    string
		input   string
		want    Person
		wantErr bool
	}{
		{
			name: "nested struct with wrapped values",
			input: `{
				"name": {"type": "string", "value": "John"},
				"address": {
					"street": {"type": "string", "value": "123 Main St"},
					"city": {"type": "string", "value": "New York"}
				}
			}`,
			want: Person{
				Name: "John",
				Address: Address{
					Street: "123 Main St",
					City:   "New York",
				},
			},
			wantErr: false,
		},
		{
			name: "deeply nested wrapped values",
			input: `{
				"name": {"type": "string", "value": "Alice"},
				"address": {"type": "object", "value": {
					"street": {"type": "string", "value": "456 Oak Ave"},
					"city": {"type": "string", "value": "Boston"}
				}}
			}`,
			want: Person{
				Name: "Alice",
				Address: Address{
					Street: "456 Oak Ave",
					City:   "Boston",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[Person](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && (got.Name != tt.want.Name || got.Address != tt.want.Address) {
				t.Errorf("ParseStringAs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_LegitimateTypeValueFields(t *testing.T) {
	// Test that objects that legitimately have "type" and "value" fields work correctly
	type SchemaField struct {
		Type  string      `json:"type"`
		Value interface{} `json:"value"`
	}

	tests := []struct {
		name    string
		input   string
		want    SchemaField
		wantErr bool
	}{
		{
			name:    "legitimate type/value object",
			input:   `{"type": "string", "value": "hello"}`,
			want:    SchemaField{Type: "string", Value: "hello"},
			wantErr: false,
		},
		{
			name:    "legitimate with numeric value",
			input:   `{"type": "integer", "value": 42}`,
			want:    SchemaField{Type: "integer", Value: float64(42)}, // JSON numbers unmarshal to float64
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[SchemaField](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Type != tt.want.Type {
				t.Errorf("ParseStringAs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_SchemaWrappedMap(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "map with wrapped values",
			input: `{
				"key1": {"type": "string", "value": "value1"},
				"key2": {"type": "string", "value": "value2"}
			}`,
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[map[string]string](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				for k, v := range tt.want {
					if got[k] != v {
						t.Errorf("ParseStringAs()[%s] = %v, want %v", k, got[k], v)
					}
				}
			}
		})
	}
}

func TestExtractJSONCandidates(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple object",
			input:    `{"name":"John"}`,
			expected: []string{`{"name":"John"}`},
		},
		{
			name:     "simple array",
			input:    `[1,2,3]`,
			expected: []string{`[1,2,3]`},
		},
		{
			name:     "text before JSON",
			input:    "Here is the result:\n{\"name\":\"John\"}",
			expected: []string{`{"name":"John"}`},
		},
		{
			name:     "text after JSON",
			input:    "{\"name\":\"John\"}\nHope this helps!",
			expected: []string{`{"name":"John"}`},
		},
		{
			name:     "text before and after JSON",
			input:    "The result is:\n{\"name\":\"John\"}\nThank you!",
			expected: []string{`{"name":"John"}`},
		},
		{
			name:     "multiple JSON objects",
			input:    `{"first":1} and {"second":2}`,
			expected: []string{`{"first":1}`, `{"second":2}`},
		},
		{
			name:     "nested JSON",
			input:    `{"outer":{"inner":"value"}}`,
			expected: []string{`{"outer":{"inner":"value"}}`, `{"inner":"value"}`},
		},
		{
			name:     "JSON with escaped quotes",
			input:    `{"text":"He said \"hello\""}`,
			expected: []string{`{"text":"He said \"hello\""}`},
		},
		{
			name:     "array with objects",
			input:    `[{"id":1},{"id":2}]`,
			expected: []string{`[{"id":1},{"id":2}]`, `{"id":1}`, `{"id":2}`},
		},
		{
			name:     "no JSON",
			input:    "This is just plain text",
			expected: []string{},
		},
		{
			name:     "incomplete JSON ignored",
			input:    "Here is incomplete: {\"name\":",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONCandidates(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("extractJSONCandidates() got %d candidates, want %d\nGot: %v\nWant: %v",
					len(got), len(tt.expected), got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("extractJSONCandidates()[%d] = %v, want %v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestParseStringAs_LLMNarrativeText(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		input   string
		want    Person
		wantErr bool
	}{
		{
			name: "text before JSON",
			input: `Here is the person data you requested:
{"name":"John","age":30}`,
			want:    Person{Name: "John", Age: 30},
			wantErr: false,
		},
		{
			name: "text after JSON",
			input: `{"name":"Jane","age":25}
Hope this helps!`,
			want:    Person{Name: "Jane", Age: 25},
			wantErr: false,
		},
		{
			name: "text before and after JSON",
			input: `Let me provide the data:
{"name":"Bob","age":35}
Is this what you needed?`,
			want:    Person{Name: "Bob", Age: 35},
			wantErr: false,
		},
		{
			name: "multiline narrative with JSON",
			input: `I found the information.
The person details are as follows:
{"name":"Alice","age":28}
Let me know if you need anything else.`,
			want:    Person{Name: "Alice", Age: 28},
			wantErr: false,
		},
		{
			name: "JSON without code block markdown",
			input: `Sure, here's the result:
{
  "name": "Charlie",
  "age": 40
}`,
			want:    Person{Name: "Charlie", Age: 40},
			wantErr: false,
		},
		{
			name: "malformed JSON with narrative (should repair)",
			input: `Here you go:
{name: 'David', age: 45}`,
			want:    Person{Name: "David", Age: 45},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[Person](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseStringAs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_TypeMismatch_ArrayToStruct(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		input   string
		want    Person
		wantErr bool
	}{
		{
			name:    "array with single object - should extract first element",
			input:   `[{"name":"John","age":30}]`,
			want:    Person{Name: "John", Age: 30},
			wantErr: false,
		},
		{
			name:    "array with multiple objects - should extract first element",
			input:   `[{"name":"Jane","age":25},{"name":"Bob","age":35}]`,
			want:    Person{Name: "Jane", Age: 25},
			wantErr: false,
		},
		{
			name: "narrative text with array - should extract first element",
			input: `Here are the results:
[{"name":"Alice","age":28}]`,
			want:    Person{Name: "Alice", Age: 28},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[Person](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseStringAs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseStringAs_TypeMismatch_ObjectToArray(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name    string
		input   string
		want    []Person
		wantErr bool
	}{
		{
			name:    "single object - should wrap in array",
			input:   `{"name":"John","age":30}`,
			want:    []Person{{Name: "John", Age: 30}},
			wantErr: false,
		},
		{
			name: "narrative text with single object - should wrap in array",
			input: `Here is the person:
{"name":"Jane","age":25}`,
			want:    []Person{{Name: "Jane", Age: 25}},
			wantErr: false,
		},
		{
			name:    "proper array - should parse normally",
			input:   `[{"name":"Bob","age":35},{"name":"Alice","age":28}]`,
			want:    []Person{{Name: "Bob", Age: 35}, {Name: "Alice", Age: 28}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[[]Person](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("ParseStringAs() length = %d, want %d", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("ParseStringAs()[%d] = %+v, want %+v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestParseStringAs_MultipleJSONObjects(t *testing.T) {
	type Result struct {
		Value int `json:"value"`
	}

	tests := []struct {
		name    string
		input   string
		want    Result
		wantErr bool
	}{
		{
			name:    "multiple JSON - first is valid",
			input:   `{"value":1} and {"value":2}`,
			want:    Result{Value: 1},
			wantErr: false,
		},
		{
			// Note: If first JSON is syntactically valid but has wrong fields,
			// Go's json.Unmarshal will succeed (fields are optional), so we use that JSON.
			// "First that doesn't fail" means first that doesn't have a parsing error.
			name: "narrative with multiple JSON - use first valid",
			input: `I have two options:
Option 1: {"value":10}
Option 2: {"value":20}
I recommend the first one.`,
			want:    Result{Value: 10},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStringAs[Result](tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStringAs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseStringAs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
