package slogobs

import (
	"io"
	"log/slog"
	"os"
)

// Option is a functional option for configuring the Observer.
type Option func(*config)

// config holds the configuration for creating an Observer.
type config struct {
	format Format
	level  slog.Level
	output io.Writer
	colors bool
	logger *slog.Logger // If provided, use this logger directly (bypass custom handler)
}

// WithFormat sets the log output format.
func WithFormat(format Format) Option {
	return func(c *config) {
		c.format = format
	}
}

// WithLevel sets the minimum log level.
func WithLevel(level slog.Level) Option {
	return func(c *config) {
		c.level = level
	}
}

// WithOutput sets the output writer for logs.
func WithOutput(output io.Writer) Option {
	return func(c *config) {
		c.output = output
	}
}

// WithColors enables or disables ANSI color codes.
// Only applies to compact and pretty formats.
func WithColors(enabled bool) Option {
	return func(c *config) {
		c.colors = enabled
	}
}

// WithLogger uses an existing slog.Logger instead of creating a custom handler.
// This option takes precedence over format/level/output/colors options.
func WithLogger(logger *slog.Logger) Option {
	return func(c *config) {
		c.logger = logger
	}
}

// defaultConfig returns the default configuration.
func defaultConfig() *config {
	return &config{
		format: GetFormatFromEnv(),
		level:  GetLogLevelFromEnv(),
		output: os.Stdout,
		colors: false, // Auto-detected by handler if output is a terminal
		logger: nil,
	}
}

// applyOptions applies the given options to the config.
func applyOptions(opts ...Option) *config {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}
