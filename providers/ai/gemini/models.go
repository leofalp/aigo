package gemini

import "encoding/json"

/*
	GEMINI API - REQUEST TYPES
*/

// generateContentRequest represents the request to Gemini's generateContent endpoint.
type generateContentRequest struct {
	Contents          []content          `json:"contents"`
	SystemInstruction *systemInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig  *generationConfig  `json:"generationConfig,omitempty"`
	Tools             []tool             `json:"tools,omitempty"`
	ToolConfig        *toolConfig        `json:"toolConfig,omitempty"`
	SafetySettings    []safetySetting    `json:"safetySettings,omitempty"`
}

// systemInstruction represents the system instruction for Gemini.
type systemInstruction struct {
	Parts []part `json:"parts"`
}

// content represents a content block with role and parts.
type content struct {
	Role  string `json:"role,omitempty"` // "user" or "model"
	Parts []part `json:"parts"`
}

// part represents a content part (text, function call, function response, inline data, file data, etc.).
type part struct {
	Text             string            `json:"text,omitempty"`
	Thought          bool              `json:"thought,omitempty"` // true if this part contains a thinking/reasoning summary
	FunctionCall     *functionCall     `json:"functionCall,omitempty"`
	FunctionResponse *functionResponse `json:"functionResponse,omitempty"`
	InlineData       *inlineData       `json:"inlineData,omitempty"` // For multimodal content (images, audio, video, documents)
	FileData         *fileData         `json:"fileData,omitempty"`   // For URI-referenced multimodal content
}

// inlineData represents inline binary data (e.g., base64-encoded images, audio, video).
type inlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// fileData represents a file reference by MIME type and URI.
// Used for URI-based multimodal content (e.g., Google Cloud Storage URIs, uploaded file URIs).
type fileData struct {
	MimeType string `json:"mimeType"`
	FileURI  string `json:"fileUri"`
}

// functionCall represents a function call from the model.
type functionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

// functionResponse represents a response to a function call.
type functionResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

// generationConfig represents generation parameters for Gemini.
type generationConfig struct {
	Temperature        *float64        `json:"temperature,omitempty"`
	TopP               *float64        `json:"topP,omitempty"`
	TopK               *int            `json:"topK,omitempty"`
	MaxOutputTokens    *int            `json:"maxOutputTokens,omitempty"`
	StopSequences      []string        `json:"stopSequences,omitempty"`
	ResponseMimeType   string          `json:"responseMimeType,omitempty"`
	ResponseSchema     json.RawMessage `json:"responseSchema,omitempty"`
	ResponseModalities []string        `json:"responseModalities,omitempty"` // Output modalities (e.g., ["TEXT", "IMAGE"])
	ThinkingConfig     *thinkingConfig `json:"thinkingConfig,omitempty"`
	CandidateCount     *int            `json:"candidateCount,omitempty"`
	PresencePenalty    *float64        `json:"presencePenalty,omitempty"`
	FrequencyPenalty   *float64        `json:"frequencyPenalty,omitempty"`
}

// thinkingConfig represents the thinking/reasoning configuration for Gemini.
type thinkingConfig struct {
	ThinkingBudget  *int `json:"thinkingBudget,omitempty"`
	IncludeThoughts bool `json:"includeThoughts,omitempty"`
}

// tool represents a tool definition for Gemini.
type tool struct {
	GoogleSearch         *googleSearchTool     `json:"googleSearch,omitempty"`
	URLContext           *urlContextTool       `json:"urlContext,omitempty"`
	CodeExecution        *codeExecutionTool    `json:"codeExecution,omitempty"`
	FunctionDeclarations []functionDeclaration `json:"functionDeclarations,omitempty"`
}

// googleSearchTool represents the Google Search grounding tool.
type googleSearchTool struct{}

// urlContextTool represents the URL context grounding tool.
type urlContextTool struct{}

// codeExecutionTool represents the code execution sandbox tool.
type codeExecutionTool struct{}

// functionDeclaration represents a user-defined function declaration.
type functionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// toolConfig represents tool configuration options.
type toolConfig struct {
	FunctionCallingConfig *functionCallingConfig `json:"functionCallingConfig,omitempty"`
}

// functionCallingConfig represents function calling configuration.
type functionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"` // "AUTO", "ANY", "NONE"
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

// safetySetting represents a safety setting for content filtering.
type safetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

/*
	GEMINI API - RESPONSE TYPES
*/

// generateContentResponse represents the response from Gemini's generateContent endpoint.
type generateContentResponse struct {
	Candidates     []candidate     `json:"candidates,omitempty"`
	PromptFeedback *promptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *usageMetadata  `json:"usageMetadata,omitempty"`
	ModelVersion   string          `json:"modelVersion,omitempty"`
}

// candidate represents a response candidate.
type candidate struct {
	Content           *content           `json:"content,omitempty"`
	FinishReason      string             `json:"finishReason,omitempty"`
	SafetyRatings     []safetyRating     `json:"safetyRatings,omitempty"`
	CitationMetadata  *citationMetadata  `json:"citationMetadata,omitempty"`
	Index             int                `json:"index,omitempty"`
	GroundingMetadata *groundingMetadata `json:"groundingMetadata,omitempty"`
}

// safetyRating represents a safety rating for generated content.
type safetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
	Blocked     bool   `json:"blocked,omitempty"`
}

// citationMetadata represents citation information.
type citationMetadata struct {
	CitationSources []citationSource `json:"citationSources,omitempty"`
}

// citationSource represents a citation source.
type citationSource struct {
	StartIndex int    `json:"startIndex,omitempty"`
	EndIndex   int    `json:"endIndex,omitempty"`
	URI        string `json:"uri,omitempty"`
	License    string `json:"license,omitempty"`
}

// groundingMetadata represents grounding information from Google Search.
type groundingMetadata struct {
	SearchEntryPoint  *searchEntryPoint  `json:"searchEntryPoint,omitempty"`
	GroundingChunks   []groundingChunk   `json:"groundingChunks,omitempty"`
	GroundingSupports []groundingSupport `json:"groundingSupports,omitempty"`
	WebSearchQueries  []string           `json:"webSearchQueries,omitempty"`
}

// searchEntryPoint represents a search entry point.
type searchEntryPoint struct {
	RenderedContent string `json:"renderedContent,omitempty"`
	SDKBlob         string `json:"sdkBlob,omitempty"`
}

// groundingChunk represents a grounding chunk.
type groundingChunk struct {
	Web *webChunk `json:"web,omitempty"`
}

// webChunk represents a web chunk.
type webChunk struct {
	URI   string `json:"uri,omitempty"`
	Title string `json:"title,omitempty"`
}

// groundingSupport represents grounding support information.
type groundingSupport struct {
	Segment               *segment  `json:"segment,omitempty"`
	GroundingChunkIndices []int     `json:"groundingChunkIndices,omitempty"`
	ConfidenceScores      []float64 `json:"confidenceScores,omitempty"`
}

// segment represents a segment of text.
type segment struct {
	StartIndex int    `json:"startIndex,omitempty"`
	EndIndex   int    `json:"endIndex,omitempty"`
	Text       string `json:"text,omitempty"`
}

// promptFeedback represents feedback about the prompt.
type promptFeedback struct {
	BlockReason   string         `json:"blockReason,omitempty"`
	SafetyRatings []safetyRating `json:"safetyRatings,omitempty"`
}

// usageMetadata represents token usage information.
type usageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount    int `json:"candidatesTokenCount,omitempty"`
	TotalTokenCount         int `json:"totalTokenCount,omitempty"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount,omitempty"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
}
