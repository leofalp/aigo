package observability

import (
	"context"
	"fmt"
	"time"
)

// Provider is the main interface for observability (tracing, metrics, logging)
type Provider interface {
	Tracer
	Metrics
	Logger
}

// --- TRACING (Distributed Tracing) ---

// Tracer provides distributed tracing capabilities
type Tracer interface {
	// StartSpan starts a new span
	StartSpan(ctx context.Context, name string, attrs ...Attribute) (context.Context, Span)
}

// Span represents a single unit of work
type Span interface {
	// End completes the span
	End()
	// SetAttributes adds attributes to the span
	SetAttributes(attrs ...Attribute)
	// SetStatus sets the span status
	SetStatus(code StatusCode, description string)
	// RecordError records an error
	RecordError(err error)
	// AddEvent adds an event to the span
	AddEvent(name string, attrs ...Attribute)
}

// StatusCode represents the status of a span
type StatusCode int

const (
	StatusUnset StatusCode = iota
	StatusOK
	StatusError
)

// --- METRICS ---

// Metrics provides metrics collection capabilities
type Metrics interface {
	// Counter creates or retrieves a counter metric
	Counter(name string) Counter
	// Histogram creates or retrieves a histogram metric
	Histogram(name string) Histogram
}

// Counter is a monotonically increasing metric
type Counter interface {
	Add(ctx context.Context, value int64, attrs ...Attribute)
}

// Histogram records distribution of values
type Histogram interface {
	Record(ctx context.Context, value float64, attrs ...Attribute)
}

// --- LOGGING (Structured Logging) ---

// Logger provides structured logging capabilities
type Logger interface {
	Trace(ctx context.Context, msg string, attrs ...Attribute)
	Debug(ctx context.Context, msg string, attrs ...Attribute)
	Info(ctx context.Context, msg string, attrs ...Attribute)
	Warn(ctx context.Context, msg string, attrs ...Attribute)
	Error(ctx context.Context, msg string, attrs ...Attribute)
}

// --- ATTRIBUTES (Key-Value pairs) ---

// Attribute represents a key-value pair for metadata
type Attribute struct {
	Key   string
	Value interface{}
}

// String creates a string attribute
func String(key, value string) Attribute {
	return Attribute{Key: key, Value: value}
}

// Int creates an integer attribute
func Int(key string, value int) Attribute {
	return Attribute{Key: key, Value: value}
}

// Int64 creates an int64 attribute
func Int64(key string, value int64) Attribute {
	return Attribute{Key: key, Value: value}
}

// Float64 creates a float64 attribute
func Float64(key string, value float64) Attribute {
	return Attribute{Key: key, Value: value}
}

// Bool creates a boolean attribute
func Bool(key string, value bool) Attribute {
	return Attribute{Key: key, Value: value}
}

// Duration creates a duration attribute
func Duration(key string, value time.Duration) Attribute {
	return Attribute{Key: key, Value: value}
}

// Error creates an error attribute
func Error(err error) Attribute {
	if err == nil {
		return Attribute{Key: "error", Value: ""}
	}
	return Attribute{Key: "error", Value: err.Error()}
}

// --- UTILITIES ---

const (
	// DefaultMaxStringLength is the default maximum length for truncated strings
	DefaultMaxStringLength = 500
)

// TruncateString truncates a string to maxLen characters, adding a suffix with the original length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 0 {
		maxLen = DefaultMaxStringLength
	}
	return fmt.Sprintf("%s... (truncated, total: %d chars)", s[:maxLen], len(s))
}

// TruncateStringDefault truncates a string using DefaultMaxStringLength
func TruncateStringDefault(s string) string {
	return TruncateString(s, DefaultMaxStringLength)
}
