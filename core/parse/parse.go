package parse

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"github.com/kaptinlin/jsonrepair"
)

// ParseStringAs attempts to parse a string into the specified type T.
// For primitive types (string, bool, int, uint, float), it performs direct conversion.
// For complex types (structs, maps, slices), it attempts JSON unmarshaling.
// If JSON unmarshaling fails, it will attempt to repair the JSON string using jsonrepair
// and retry the unmarshaling operation.
// TODO
//
// Type parameters:
//   - T: The target type to parse the string into
//
// Parameters:
//   - content: The string content to parse
//
// Returns:
//   - T: The parsed value of type T
//   - error: An error if parsing fails after all attempts
//
// Example usage:
//
//	type Person struct {
//	    Name string `json:"name"`
//	    Age  int    `json:"age"`
//	}
//
//	// Parse a valid JSON string
//	person, err := ParseStringAs[Person](`{"name":"John","age":30}`)
//
//	// Parse an invalid JSON string (will be auto-repaired)
//	person, err := ParseStringAs[Person](`{name: 'John', age: 30}`)
//
//	// Parse primitive types
//	num, err := ParseStringAs[int]("42")
//	flag, err := ParseStringAs[bool]("true")
func ParseStringAs[T any](content string) (T, error) {
	var result T

	switch reflect.TypeFor[T]().Kind() {
	case reflect.String:
		// For string type, try direct parsing first
		// If content looks like JSON, try to unwrap schema values
		if len(content) > 0 && content[0] == '{' {
			if unwrapped, err := tryUnwrapPrimitive(content); err == nil {
				reflect.ValueOf(&result).Elem().SetString(unwrapped)
				return result, nil
			}
		}
		// Return content as-is via reflection
		reflect.ValueOf(&result).Elem().SetString(content)
		return result, nil

	case reflect.Bool:
		val, err := strconv.ParseBool(content)
		if err != nil {
			// Try to unwrap if it's a schema-wrapped value
			if unwrapped, unwrapErr := tryUnwrapPrimitive(content); unwrapErr == nil {
				val, err = strconv.ParseBool(unwrapped)
				if err == nil {
					reflect.ValueOf(&result).Elem().SetBool(val)
					return result, nil
				}
			}
			return result, fmt.Errorf("failed to parse content as bool: %w", err)
		}
		reflect.ValueOf(&result).Elem().SetBool(val)
		return result, nil

	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(content, 64)
		if err != nil {
			// Try to unwrap if it's a schema-wrapped value
			if unwrapped, unwrapErr := tryUnwrapPrimitive(content); unwrapErr == nil {
				val, err = strconv.ParseFloat(unwrapped, 64)
				if err == nil {
					reflect.ValueOf(&result).Elem().SetFloat(val)
					return result, nil
				}
			}
			return result, fmt.Errorf("failed to parse content as float: %w", err)
		}
		reflect.ValueOf(&result).Elem().SetFloat(val)
		return result, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := strconv.ParseInt(content, 10, 64)
		if err != nil {
			// Try to unwrap if it's a schema-wrapped value
			if unwrapped, unwrapErr := tryUnwrapPrimitive(content); unwrapErr == nil {
				val, err = strconv.ParseInt(unwrapped, 10, 64)
				if err == nil {
					reflect.ValueOf(&result).Elem().SetInt(val)
					return result, nil
				}
			}
			return result, fmt.Errorf("failed to parse content as int: %w", err)
		}
		reflect.ValueOf(&result).Elem().SetInt(val)
		return result, nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(content, 10, 64)
		if err != nil {
			// Try to unwrap if it's a schema-wrapped value
			if unwrapped, unwrapErr := tryUnwrapPrimitive(content); unwrapErr == nil {
				val, err = strconv.ParseUint(unwrapped, 10, 64)
				if err == nil {
					reflect.ValueOf(&result).Elem().SetUint(val)
					return result, nil
				}
			}
			return result, fmt.Errorf("failed to parse content as uint: %w", err)
		}
		reflect.ValueOf(&result).Elem().SetUint(val)
		return result, nil

	default:
		// For structs, slices, maps, and other complex types, use JSON unmarshaling
		err := json.Unmarshal([]byte(content), &result)
		if err != nil {
			// If JSON unmarshaling fails, attempt to repair the JSON and retry
			repairedJSON, repairErr := jsonrepair.JSONRepair(content)
			if repairErr != nil {
				return result, fmt.Errorf("failed to unmarshal content as %T and failed to repair JSON: unmarshal error: %w, repair error: %v", result, err, repairErr)
			}

			// Retry unmarshaling with repaired JSON
			err = json.Unmarshal([]byte(repairedJSON), &result)
			if err != nil {
				// If still failing, try to unwrap schema-like {type, value} structures
				// This handles cases where LLMs confuse JSON schema with actual data
				unwrapped, unwrapErr := unwrapSchemaValues(repairedJSON)
				if unwrapErr == nil {
					err = json.Unmarshal([]byte(unwrapped), &result)
					if err == nil {
						return result, nil
					}
				}

				return result, fmt.Errorf("failed to unmarshal repaired JSON as %T: %w (original content: %s, repaired: %s)", result, err, content, repairedJSON)
			}
		}
		return result, nil
	}
}

// tryUnwrapPrimitive attempts to unwrap a primitive value from a schema-like structure.
// Returns the string representation of the unwrapped value.
func tryUnwrapPrimitive(content string) (string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return "", err
	}

	// Check if this has the schema pattern: {"type": "...", "value": ...}
	if _, hasType := data["type"]; hasType {
		if value, hasValue := data["value"]; hasValue && len(data) == 2 {
			// Convert value to string representation
			switch v := value.(type) {
			case string:
				return v, nil
			case float64:
				return fmt.Sprintf("%v", v), nil
			case bool:
				return fmt.Sprintf("%v", v), nil
			default:
				// For complex types, marshal back to JSON
				bytes, err := json.Marshal(v)
				if err != nil {
					return "", err
				}
				return string(bytes), nil
			}
		}
	}

	return "", fmt.Errorf("not a schema-wrapped value")
}

// unwrapSchemaValues attempts to detect and unwrap values that are wrapped
// in a schema-like structure with "type" and "value" fields.
// This is a common error when LLMs confuse JSON schema definitions with actual data.
//
// Example input:
//
//	{"name": {"type": "string", "value": "John"}, "age": {"type": "integer", "value": 30}}
//
// Example output:
//
//	{"name": "John", "age": 30}
func unwrapSchemaValues(jsonStr string) (string, error) {
	// Try to unmarshal as generic interface to inspect structure
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", err
	}

	// Recursively unwrap the data
	unwrapped := recursiveUnwrap(data)

	// Marshal back to JSON string
	result, err := json.Marshal(unwrapped)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// recursiveUnwrap recursively processes data structures to unwrap schema-like values
func recursiveUnwrap(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		// Check if this map has the schema pattern: {"type": "...", "value": ...}
		if _, hasType := v["type"]; hasType {
			if value, hasValue := v["value"]; hasValue && len(v) == 2 {
				// This looks like a schema wrapper, unwrap it
				// Recursively unwrap in case the value itself contains wrapped data
				return recursiveUnwrap(value)
			}
		}

		// Not a schema wrapper, process each field recursively
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = recursiveUnwrap(val)
		}
		return result

	case []interface{}:
		// Process array elements recursively
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = recursiveUnwrap(val)
		}
		return result

	default:
		// Primitive types, return as-is
		return data
	}
}
