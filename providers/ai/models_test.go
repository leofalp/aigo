package ai

import (
	"encoding/json"
	"testing"
)

// TestNewPart_Constructors exercises all nine ContentPart constructors using a
// table-driven approach. Each row verifies that the correct ContentType is set,
// the right embedded struct is populated with the expected MimeType, and the
// Data or URI field (depending on inline vs URI variant) contains the input value.
func TestNewPart_Constructors(t *testing.T) {
	tests := []struct {
		name         string
		buildPart    func() ContentPart
		wantType     ContentType
		wantText     string // only for text parts
		wantMimeType string // expected MimeType in the embedded struct
		wantData     string // expected Data field (empty for URI variants)
		wantURI      string // expected URI field (empty for inline variants)
	}{
		{
			name:      "NewTextPart sets Type and Text",
			buildPart: func() ContentPart { return NewTextPart("hello world") },
			wantType:  ContentTypeText,
			wantText:  "hello world",
		},
		{
			name:         "NewImagePart sets Type, MimeType, and Data",
			buildPart:    func() ContentPart { return NewImagePart("image/png", "base64img") },
			wantType:     ContentTypeImage,
			wantMimeType: "image/png",
			wantData:     "base64img",
		},
		{
			name:         "NewImagePartFromURI sets Type, MimeType, and URI",
			buildPart:    func() ContentPart { return NewImagePartFromURI("image/jpeg", "https://example.com/photo.jpg") },
			wantType:     ContentTypeImage,
			wantMimeType: "image/jpeg",
			wantURI:      "https://example.com/photo.jpg",
		},
		{
			name:         "NewAudioPart sets Type, MimeType, and Data",
			buildPart:    func() ContentPart { return NewAudioPart("audio/wav", "base64audio") },
			wantType:     ContentTypeAudio,
			wantMimeType: "audio/wav",
			wantData:     "base64audio",
		},
		{
			name:         "NewAudioPartFromURI sets Type, MimeType, and URI",
			buildPart:    func() ContentPart { return NewAudioPartFromURI("audio/mp3", "gs://bucket/audio.mp3") },
			wantType:     ContentTypeAudio,
			wantMimeType: "audio/mp3",
			wantURI:      "gs://bucket/audio.mp3",
		},
		{
			name:         "NewVideoPart sets Type, MimeType, and Data",
			buildPart:    func() ContentPart { return NewVideoPart("video/mp4", "base64video") },
			wantType:     ContentTypeVideo,
			wantMimeType: "video/mp4",
			wantData:     "base64video",
		},
		{
			name:         "NewVideoPartFromURI sets Type, MimeType, and URI",
			buildPart:    func() ContentPart { return NewVideoPartFromURI("video/webm", "https://cdn.example.com/clip.webm") },
			wantType:     ContentTypeVideo,
			wantMimeType: "video/webm",
			wantURI:      "https://cdn.example.com/clip.webm",
		},
		{
			name:         "NewDocumentPart sets Type, MimeType, and Data",
			buildPart:    func() ContentPart { return NewDocumentPart("application/pdf", "base64pdf") },
			wantType:     ContentTypeDocument,
			wantMimeType: "application/pdf",
			wantData:     "base64pdf",
		},
		{
			name:         "NewDocumentPartFromURI sets Type, MimeType, and URI",
			buildPart:    func() ContentPart { return NewDocumentPartFromURI("text/plain", "file:///tmp/notes.txt") },
			wantType:     ContentTypeDocument,
			wantMimeType: "text/plain",
			wantURI:      "file:///tmp/notes.txt",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			part := testCase.buildPart()

			if part.Type != testCase.wantType {
				t.Errorf("Type = %q, want %q", part.Type, testCase.wantType)
			}

			// Text parts only carry the Text field; no embedded media struct.
			if testCase.wantType == ContentTypeText {
				if part.Text != testCase.wantText {
					t.Errorf("Text = %q, want %q", part.Text, testCase.wantText)
				}
				return
			}

			// For media parts, extract the MimeType, Data, and URI from the
			// correct embedded struct based on the expected ContentType.
			var gotMimeType, gotData, gotURI string
			switch testCase.wantType {
			case ContentTypeImage:
				if part.Image == nil {
					t.Fatal("Image is nil, expected non-nil ImageData")
				}
				gotMimeType = part.Image.MimeType
				gotData = part.Image.Data
				gotURI = part.Image.URI
			case ContentTypeAudio:
				if part.Audio == nil {
					t.Fatal("Audio is nil, expected non-nil AudioData")
				}
				gotMimeType = part.Audio.MimeType
				gotData = part.Audio.Data
				gotURI = part.Audio.URI
			case ContentTypeVideo:
				if part.Video == nil {
					t.Fatal("Video is nil, expected non-nil VideoData")
				}
				gotMimeType = part.Video.MimeType
				gotData = part.Video.Data
				gotURI = part.Video.URI
			case ContentTypeDocument:
				if part.Document == nil {
					t.Fatal("Document is nil, expected non-nil DocumentData")
				}
				gotMimeType = part.Document.MimeType
				gotData = part.Document.Data
				gotURI = part.Document.URI
			}

			if gotMimeType != testCase.wantMimeType {
				t.Errorf("MimeType = %q, want %q", gotMimeType, testCase.wantMimeType)
			}
			if gotData != testCase.wantData {
				t.Errorf("Data = %q, want %q", gotData, testCase.wantData)
			}
			if gotURI != testCase.wantURI {
				t.Errorf("URI = %q, want %q", gotURI, testCase.wantURI)
			}
		})
	}
}

// TestNewToolResultSuccess verifies that NewToolResultSuccess produces a
// ToolResult with Success=true and the provided data stored in the Data field.
func TestNewToolResultSuccess(t *testing.T) {
	payload := map[string]string{"city": "Buenos Aires", "country": "Argentina"}
	result := NewToolResultSuccess(payload)

	if !result.Success {
		t.Error("Success = false, want true")
	}
	if result.Error != "" {
		t.Errorf("Error = %q, want empty string", result.Error)
	}
	if result.Message != "" {
		t.Errorf("Message = %q, want empty string", result.Message)
	}

	// Verify the data is the same reference we passed in.
	data, ok := result.Data.(map[string]string)
	if !ok {
		t.Fatalf("Data type = %T, want map[string]string", result.Data)
	}
	if data["city"] != "Buenos Aires" {
		t.Errorf("Data[\"city\"] = %q, want %q", data["city"], "Buenos Aires")
	}
	if data["country"] != "Argentina" {
		t.Errorf("Data[\"country\"] = %q, want %q", data["country"], "Argentina")
	}
}

// TestNewToolResultError verifies that NewToolResultError produces a ToolResult
// with Success=false and the error type and message populated correctly.
func TestNewToolResultError(t *testing.T) {
	result := NewToolResultError("tool_not_found", "no tool named 'foobar' is registered")

	if result.Success {
		t.Error("Success = true, want false")
	}
	if result.Error != "tool_not_found" {
		t.Errorf("Error = %q, want %q", result.Error, "tool_not_found")
	}
	if result.Message != "no tool named 'foobar' is registered" {
		t.Errorf("Message = %q, want %q", result.Message, "no tool named 'foobar' is registered")
	}
	if result.Data != nil {
		t.Errorf("Data = %v, want nil", result.Data)
	}
}

// TestToJSON_ValidStruct verifies that ToJSON marshals a successful ToolResult
// into valid JSON that round-trips back to the expected key-value data.
func TestToJSON_ValidStruct(t *testing.T) {
	result := NewToolResultSuccess(map[string]string{"key": "val"})
	jsonStr, err := result.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() returned unexpected error: %v", err)
	}

	// Verify the output is valid JSON by unmarshaling into a generic map.
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("ToJSON() produced invalid JSON: %v\nraw output: %s", err, jsonStr)
	}

	// Confirm top-level success field.
	success, ok := parsed["success"].(bool)
	if !ok || !success {
		t.Errorf("parsed[\"success\"] = %v, want true", parsed["success"])
	}

	// Confirm the nested data object contains the expected key-value pair.
	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("parsed[\"data\"] type = %T, want map[string]interface{}", parsed["data"])
	}
	if data["key"] != "val" {
		t.Errorf("parsed[\"data\"][\"key\"] = %v, want %q", data["key"], "val")
	}
}

// TestToJSON_NestedStruct verifies that ToJSON correctly marshals a ToolResult
// containing a nested struct as data, and that the output round-trips back to
// the original values when parsed.
func TestToJSON_NestedStruct(t *testing.T) {
	type address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}
	type person struct {
		Name    string  `json:"name"`
		Age     int     `json:"age"`
		Address address `json:"address"`
	}

	input := person{
		Name: "Ada Lovelace",
		Age:  36,
		Address: address{
			Street: "St James's Square",
			City:   "London",
		},
	}

	result := NewToolResultSuccess(input)
	jsonStr, err := result.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() returned unexpected error: %v", err)
	}

	// Round-trip: unmarshal back into the ToolResult structure and verify fields.
	var roundTripped ToolResult
	if err := json.Unmarshal([]byte(jsonStr), &roundTripped); err != nil {
		t.Fatalf("failed to unmarshal ToJSON() output: %v", err)
	}

	if !roundTripped.Success {
		t.Error("round-tripped Success = false, want true")
	}

	// The Data field deserializes as map[string]interface{} since we lose the
	// concrete Go type through the JSON round-trip.
	data, ok := roundTripped.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("round-tripped Data type = %T, want map[string]interface{}", roundTripped.Data)
	}
	if data["name"] != "Ada Lovelace" {
		t.Errorf("data[\"name\"] = %v, want %q", data["name"], "Ada Lovelace")
	}

	// json.Unmarshal decodes numbers as float64 by default.
	if age, ok := data["age"].(float64); !ok || age != 36 {
		t.Errorf("data[\"age\"] = %v, want 36", data["age"])
	}

	nestedAddr, ok := data["address"].(map[string]interface{})
	if !ok {
		t.Fatalf("data[\"address\"] type = %T, want map[string]interface{}", data["address"])
	}
	if nestedAddr["city"] != "London" {
		t.Errorf("data[\"address\"][\"city\"] = %v, want %q", nestedAddr["city"], "London")
	}
}

// TestIsBuiltinTool verifies that IsBuiltinTool correctly identifies tool names
// that are built-in pseudo-tools (prefixed with underscore) vs user-defined tools.
func TestIsBuiltinTool(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		want     bool
	}{
		{name: "_google_search is builtin", toolName: "_google_search", want: true},
		{name: "_url_context is builtin", toolName: "_url_context", want: true},
		{name: "_code_execution is builtin", toolName: "_code_execution", want: true},
		{name: "empty string is not builtin", toolName: "", want: false},
		{name: "my_tool is not builtin", toolName: "my_tool", want: false},
		{name: "search is not builtin", toolName: "search", want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := IsBuiltinTool(testCase.toolName)
			if got != testCase.want {
				t.Errorf("IsBuiltinTool(%q) = %v, want %v", testCase.toolName, got, testCase.want)
			}
		})
	}
}
