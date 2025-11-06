package utils

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
		// For string type, return content as-is via reflection
		reflect.ValueOf(&result).Elem().SetString(content)
		return result, nil

	case reflect.Bool:
		val, err := strconv.ParseBool(content)
		if err != nil {
			return result, fmt.Errorf("failed to parse content as bool: %w", err)
		}
		reflect.ValueOf(&result).Elem().SetBool(val)
		return result, nil

	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(content, 64)
		if err != nil {
			return result, fmt.Errorf("failed to parse content as float: %w", err)
		}
		reflect.ValueOf(&result).Elem().SetFloat(val)
		return result, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := strconv.ParseInt(content, 10, 64)
		if err != nil {
			return result, fmt.Errorf("failed to parse content as int: %w", err)
		}
		reflect.ValueOf(&result).Elem().SetInt(val)
		return result, nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(content, 10, 64)
		if err != nil {
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
				return result, fmt.Errorf("failed to unmarshal repaired JSON as %T: %w (original content: %s, repaired: %s)", result, err, content, repairedJSON)
			}
		}
		return result, nil
	}
}
