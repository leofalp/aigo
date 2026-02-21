package parse

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"github.com/kaptinlin/jsonrepair"
)

// extractJSONCandidates finds all potential JSON structures (objects and arrays) in a string.
// It returns a slice of candidate JSON strings in the order they appear.
// This is useful when LLMs add narrative text before or around JSON output.
//
// Parameters:
//   - content: The string that may contain JSON embedded in text
//
// Returns:
//   - []string: A slice of potential JSON strings found in the content
//
// Example:
//
//	content := "Here is the result:\n{\"name\":\"John\"}\nHope this helps!"
//	candidates := extractJSONCandidates(content)
//	// Returns: ["{\"name\":\"John\"}"]
func extractJSONCandidates(content string) []string {
	var candidates []string
	runes := []rune(content)

	// Look for opening brackets: { or [
	for i := 0; i < len(runes); i++ {
		if runes[i] != '{' && runes[i] != '[' {
			continue
		}

		// Found an opening bracket, now find its balanced closing bracket
		openChar := runes[i]
		closeChar := '}'
		if openChar == '[' {
			closeChar = ']'
		}

		depth := 0
		inString := false
		escaped := false

		for j := i; j < len(runes); j++ {
			char := runes[j]

			// Handle escape sequences in strings
			if escaped {
				escaped = false
				continue
			}

			if char == '\\' && inString {
				escaped = true
				continue
			}

			// Handle string boundaries (only double quotes are valid in JSON)
			if char == '"' {
				inString = !inString
				continue
			}

			// Skip characters inside strings
			if inString {
				continue
			}

			// Track bracket depth
			if char == openChar {
				depth++
			} else if char == closeChar {
				depth--
				if depth == 0 {
					// Found balanced closing bracket
					candidate := string(runes[i : j+1])
					candidates = append(candidates, candidate)
					break
				}
			}
		}
	}

	return candidates
}

// ParseStringAs attempts to parse content into the specified type T, applying
// increasingly aggressive recovery strategies to handle the imperfect text
// output that language models commonly produce.
//
// For primitive types (string, bool, int, uint, float) it performs direct
// string conversion via strconv, with a fallback that unwraps schema-style
// envelopes of the form {"type":"string","value":"..."}.
//
// For complex types (structs, maps, slices) it first tries standard JSON
// unmarshaling, then progressively falls back to: extracting JSON candidates
// embedded in narrative text, repairing malformed JSON via the jsonrepair
// library, unwrapping schema-wrapped fields, and reconciling type mismatches
// (array-when-struct-expected and object-when-slice-expected).
//
// Returns the zero value of T and a descriptive error if all strategies fail.
//
// Example:
//
//	type Person struct {
//	    Name string `json:"name"`
//	    Age  int    `json:"age"`
//	}
//
//	// Clean JSON
//	person, err := ParseStringAs[Person](`{"name":"John","age":30}`)
//
//	// JSON embedded in prose
//	person, err := ParseStringAs[Person]("Here is the data:\n{\"name\":\"John\",\"age\":30}")
//
//	// Malformed JSON (auto-repaired)
//	person, err := ParseStringAs[Person](`{name: 'John', age: 30}`)
//
//	// Primitive types
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
			// If JSON unmarshaling fails, try to extract JSON candidates from the content
			// This handles cases where LLMs add narrative text before/after JSON
			candidates := extractJSONCandidates(content)
			if len(candidates) == 0 {
				// No JSON candidates found, try to repair the entire content
				candidates = []string{content}
			}

			// Try each candidate in order until one succeeds
			var lastErr error
			for _, candidate := range candidates {
				// Attempt to repair the JSON candidate
				repairedJSON, repairErr := jsonrepair.JSONRepair(candidate)
				if repairErr != nil {
					lastErr = fmt.Errorf("repair error: %v", repairErr)
					continue
				}

				// Try unmarshaling with repaired JSON
				err = json.Unmarshal([]byte(repairedJSON), &result)
				if err == nil {
					return result, nil
				}

				// Try to unwrap schema-like {type, value} structures
				unwrapped, unwrapErr := unwrapSchemaValues(repairedJSON)
				if unwrapErr == nil {
					err = json.Unmarshal([]byte(unwrapped), &result)
					if err == nil {
						return result, nil
					}
				}

				// Handle type mismatches between expected type and found JSON
				targetKind := reflect.TypeFor[T]().Kind()

				// Case 1: Expected a struct/map but found an array - try first element
				if targetKind == reflect.Struct || targetKind == reflect.Map {
					var arr []json.RawMessage
					if json.Unmarshal([]byte(repairedJSON), &arr) == nil && len(arr) > 0 {
						// Try to unmarshal the first element
						if json.Unmarshal(arr[0], &result) == nil {
							return result, nil
						}
					}
				}

				// Case 2: Expected a slice but found an object - wrap in array
				if targetKind == reflect.Slice {
					wrapped := "[" + repairedJSON + "]"
					if json.Unmarshal([]byte(wrapped), &result) == nil {
						return result, nil
					}
				}

				lastErr = err
			}

			if lastErr == nil {
				lastErr = err
			}
			return result, fmt.Errorf("failed to unmarshal content as %T after trying all candidates: %w (original content: %s)", result, lastErr, content)
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
