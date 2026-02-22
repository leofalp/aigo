package slogobs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// Handler is a custom slog.Handler that supports multiple output formats.
type Handler struct {
	format Format
	level  slog.Level
	output io.Writer
	colors bool
	mu     sync.Mutex
	attrs  []slog.Attr
	groups []string
}

// HandlerOptions configures a Handler.
type HandlerOptions struct {
	// Format specifies the output format (compact, pretty, json).
	Format Format
	// Level is the minimum log level to output.
	Level slog.Level
	// Output is where logs are written (defaults to os.Stdout).
	Output io.Writer
	// Colors enables ANSI color codes (only for compact/pretty formats).
	Colors bool
}

// NewHandler creates a new Handler with the given options.
func NewHandler(opts *HandlerOptions) *Handler {
	if opts == nil {
		opts = &HandlerOptions{}
	}
	if opts.Output == nil {
		opts.Output = os.Stdout
	}
	if opts.Format == "" {
		opts.Format = FormatCompact
	}

	// Auto-detect TTY for colors if not explicitly set
	colors := opts.Colors
	if !colors && opts.Format != FormatJSON {
		if f, ok := opts.Output.(*os.File); ok {
			colors = isTerminal(f)
		}
	}

	return &Handler{
		format: opts.Format,
		level:  opts.Level,
		output: opts.Output,
		colors: colors,
		attrs:  []slog.Attr{},
		groups: []string{},
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats and writes a log record.
func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch h.format {
	case FormatPretty:
		return h.handlePretty(r)
	case FormatJSON:
		return h.handleJSON(r)
	default: // FormatCompact
		return h.handleCompact(r)
	}
}

// WithAttrs returns a new Handler with additional attributes.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := append([]slog.Attr{}, h.attrs...)
	newAttrs = append(newAttrs, attrs...)

	return &Handler{
		format: h.format,
		level:  h.level,
		output: h.output,
		colors: h.colors,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

// WithGroup returns a new Handler with a group name.
func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	newGroups := append([]string{}, h.groups...)
	newGroups = append(newGroups, name)

	return &Handler{
		format: h.format,
		level:  h.level,
		output: h.output,
		colors: h.colors,
		attrs:  h.attrs,
		groups: newGroups,
	}
}

// handleCompact formats and writes a log record in compact single-line format:
// "2006-01-02 15:04:05 LEVEL Message → {"key":"value"}"
// Attributes are JSON-encoded; colors are applied only if enabled and level colors the output.
func (h *Handler) handleCompact(r slog.Record) error {
	buf := make([]byte, 0, 256)

	// Time (without timezone)
	buf = append(buf, r.Time.Format("2006-01-02 15:04:05")...)
	buf = append(buf, ' ')

	// Level with optional color (right-aligned, 5 chars width)
	level := levelString(r.Level)
	if h.colors {
		buf = append(buf, colorForLevel(r.Level)...)
		buf = append(buf, fmt.Sprintf("%5s", level)...)
		buf = append(buf, colorReset...)
	} else {
		buf = append(buf, fmt.Sprintf("%5s", level)...)
	}
	buf = append(buf, ' ')

	// Message
	buf = append(buf, r.Message...)

	// Collect all attributes
	attrs := h.collectAttrs(r)
	if len(attrs) > 0 {
		buf = append(buf, " → "...)

		// Encode attributes as JSON
		jsonData, err := json.Marshal(attrs)
		if err != nil {
			// Fallback to key=value if JSON encoding fails
			buf = append(buf, " [json-error]"...)
		} else {
			buf = append(buf, jsonData...)
		}
	}

	buf = append(buf, '\n')
	_, err := h.output.Write(buf)
	return err
}

// handlePretty formats and writes a log record in a multi-line pretty format with tree-style indentation:
// "2025-11-03 10:40:35 🔵 DEBUG  Message\n                   ├─ key: value\n                   └─ key: value"
// Each attribute appears on a separate line with appropriate tree symbols and optional colors.
func (h *Handler) handlePretty(r slog.Record) error {
	buf := make([]byte, 0, 256)

	// Time (without timezone)
	buf = append(buf, r.Time.Format("2006-01-02 15:04:05")...)
	buf = append(buf, ' ')

	// Emoji icon for level
	buf = append(buf, emojiForLevel(r.Level)...)
	buf = append(buf, ' ')

	// Level with optional color
	level := levelString(r.Level)
	if h.colors {
		buf = append(buf, colorForLevel(r.Level)...)
		buf = append(buf, level...)
		buf = append(buf, colorReset...)
	} else {
		buf = append(buf, level...)
	}

	// Padding to align level (5 chars for level + 2 spaces)
	levelPadding := 7 - len(level)
	for i := 0; i < levelPadding; i++ {
		buf = append(buf, ' ')
	}

	// Message
	buf = append(buf, r.Message...)
	buf = append(buf, '\n')

	// Attributes on separate lines with tree-style indentation
	attrs := h.collectAttrs(r)
	if len(attrs) > 0 {
		// Count attributes for proper tree symbols
		count := 0
		total := len(attrs)

		for key, value := range attrs {
			count++
			// Use proper tree characters: ├─ for middle items, └─ for last item
			if count == total {
				buf = append(buf, "                   └─ "...)
			} else {
				buf = append(buf, "                   ├─ "...)
			}
			buf = append(buf, key...)
			buf = append(buf, ": "...)
			buf = append(buf, fmt.Sprintf("%v", value)...)
			buf = append(buf, '\n')
		}
	}

	_, err := h.output.Write(buf)
	return err
}

// handleJSON formats and writes a log record as a single JSON object:
// {"time":"2025-11-03T10:40:35","level":"DEBUG","msg":"Message","key":"value"}
// Standard fields (time, level, msg) are always included; attributes are merged at the top level.
func (h *Handler) handleJSON(r slog.Record) error {
	data := make(map[string]interface{})

	// Standard fields
	data["time"] = r.Time.Format("2006-01-02T15:04:05")
	data["level"] = levelString(r.Level)
	data["msg"] = r.Message

	// Merge attributes
	attrs := h.collectAttrs(r)
	for key, value := range attrs {
		data[key] = value
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	jsonData = append(jsonData, '\n')
	_, err = h.output.Write(jsonData)
	return err
}

// collectAttrs gathers all attributes from the handler's stored attributes and the log record
// into a single map, respecting any group prefixes defined in the handler.
func (h *Handler) collectAttrs(r slog.Record) map[string]interface{} {
	attrs := make(map[string]interface{})

	// Add handler's stored attributes
	for _, attr := range h.attrs {
		h.addAttr(attrs, attr)
	}

	// Add record's attributes
	r.Attrs(func(attr slog.Attr) bool {
		h.addAttr(attrs, attr)
		return true
	})

	return attrs
}

// addAttr adds an attribute to the map, prefixing the key with any group names if they exist.
func (h *Handler) addAttr(attrs map[string]interface{}, attr slog.Attr) {
	key := attr.Key
	if len(h.groups) > 0 {
		// Prefix with group names
		for _, group := range h.groups {
			key = group + "." + key
		}
	}

	attrs[key] = attr.Value.Any()
}

// levelString returns a string representation of the given slog.Level,
// mapping TRACE (level < Debug), DEBUG, INFO, WARN, and ERROR appropriately.
func levelString(level slog.Level) string {
	switch {
	case level < slog.LevelDebug:
		return "TRACE"
	case level < slog.LevelInfo:
		return "DEBUG"
	case level < slog.LevelWarn:
		return "INFO"
	case level < slog.LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
)

// colorForLevel returns the appropriate ANSI color code for the given slog.Level.
func colorForLevel(level slog.Level) string {
	switch {
	case level < slog.LevelDebug:
		return colorGray // TRACE
	case level < slog.LevelInfo:
		return colorBlue // DEBUG
	case level < slog.LevelWarn:
		return colorGreen // INFO
	case level < slog.LevelError:
		return colorYellow // WARN
	default:
		return colorRed // ERROR
	}
}

// emojiForLevel returns an emoji icon appropriate for the given slog.Level.
func emojiForLevel(level slog.Level) string {
	switch {
	case level < slog.LevelDebug:
		return "🔍" // TRACE
	case level < slog.LevelInfo:
		return "🔵" // DEBUG
	case level < slog.LevelWarn:
		return "🟢" // INFO
	case level < slog.LevelError:
		return "🟡" // WARN
	default:
		return "🔴" // ERROR
	}
}

// isTerminal checks whether the given file is connected to a terminal device.
// It returns false if the file is nil or if stat fails.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	fileInfo, err := f.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
