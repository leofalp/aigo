package utils

import "encoding/json"

// JSONToString converts v to its JSON representation.
func JSONToString(object interface{}, indent ...bool) string {
	var encoded []byte
	var err error
	if indent != nil && len(indent) > 0 && indent[0] {
		encoded, err = json.MarshalIndent(object, "", "  ")
	} else {
		encoded, err = json.Marshal(object)
	}
	if err != nil {
		return "{\"error\": \"failed to marshal to JSON: " + err.Error() + "\"}"
	}
	return string(encoded)
}

// ToString uses JSONToString and returns the JSON string.
// If an error occurs, it returns only the error text.
func ToString(object interface{}) string {
	return JSONToString(object)
}
