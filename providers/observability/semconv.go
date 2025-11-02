package observability

// Semantic conventions for observability attributes.
// These constants define standard attribute names to ensure consistency
// across different components of the system.

// --- LLM Provider Attributes ---

const (
	// AttrLLMProvider is the name of the LLM provider (e.g., "openai", "anthropic")
	AttrLLMProvider = "llm.provider"

	// AttrLLMModel is the model identifier (e.g., "gpt-4", "claude-3")
	AttrLLMModel = "llm.model"

	// AttrLLMEndpoint is the API endpoint URL
	AttrLLMEndpoint = "llm.endpoint"

	// AttrLLMRequestID is the unique request identifier from the provider
	AttrLLMRequestID = "llm.request.id"

	// AttrLLMResponseID is the unique response identifier from the provider
	AttrLLMResponseID = "llm.response.id"

	// AttrLLMFinishReason is the reason the generation finished
	AttrLLMFinishReason = "llm.finish_reason"

	// AttrLLMTemperature is the sampling temperature used
	AttrLLMTemperature = "llm.temperature"

	// AttrLLMMaxTokens is the maximum tokens allowed
	AttrLLMMaxTokens = "llm.max_tokens"
)

// --- Token Usage Attributes ---

const (
	// AttrLLMTokensPrompt is the number of prompt tokens
	AttrLLMTokensPrompt = "llm.tokens.prompt"

	// AttrLLMTokensCompletion is the number of completion tokens
	AttrLLMTokensCompletion = "llm.tokens.completion"

	// AttrLLMTokensTotal is the total number of tokens
	AttrLLMTokensTotal = "llm.tokens.total"
)

// --- Tool Execution Attributes ---

const (
	// AttrToolName is the name of the tool being executed
	AttrToolName = "tool.name"

	// AttrToolDescription is the tool description
	AttrToolDescription = "tool.description"

	// AttrToolInput is the tool input (serialized)
	AttrToolInput = "tool.input"

	// AttrToolOutput is the tool output (serialized)
	AttrToolOutput = "tool.output"

	// AttrToolDuration is the execution duration
	AttrToolDuration = "tool.duration"

	// AttrToolError is the error message if tool execution failed
	AttrToolError = "tool.error"
)

// --- HTTP Attributes ---

const (
	// AttrHTTPMethod is the HTTP method (GET, POST, etc.)
	AttrHTTPMethod = "http.method"

	// AttrHTTPStatusCode is the HTTP response status code
	AttrHTTPStatusCode = "http.status_code"

	// AttrHTTPURL is the full request URL
	AttrHTTPURL = "http.url"

	// AttrHTTPRequestBodySize is the request body size in bytes
	AttrHTTPRequestBodySize = "http.request.body.size"

	// AttrHTTPResponseBodySize is the response body size in bytes
	AttrHTTPResponseBodySize = "http.response.body.size"
)

// --- General Attributes ---

const (
	// AttrError is the error message
	AttrError = "error"

	// AttrErrorType is the error type/class
	AttrErrorType = "error.type"

	// AttrDuration is the operation duration
	AttrDuration = "duration"

	// AttrStatus is the operation status
	AttrStatus = "status"
)

// --- Span Names ---

const (
	// SpanClientSendMessage is the span name for client message sending
	SpanClientSendMessage = "client.send_message"

	// SpanLLMRequest is the span name for LLM API requests
	SpanLLMRequest = "llm.request"

	// SpanToolExecution is the span name for tool executions
	SpanToolExecution = "tool.execution"

	// SpanMemoryOperation is the span name for memory operations
	SpanMemoryOperation = "memory.operation"
)

// --- Event Names ---

const (
	// EventLLMRequestStart marks the start of an LLM request
	EventLLMRequestStart = "llm.request.start"

	// EventLLMRequestEnd marks the end of an LLM request
	EventLLMRequestEnd = "llm.request.end"

	// EventToolExecutionStart marks the start of tool execution
	EventToolExecutionStart = "tool.execution.start"

	// EventToolExecutionEnd marks the end of tool execution
	EventToolExecutionEnd = "tool.execution.end"

	// EventTokensReceived marks when tokens are received from LLM
	EventTokensReceived = "llm.tokens.received"
)
