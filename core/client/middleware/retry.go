package middleware

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/providers/ai"
)

// RetryConfig holds the tuning parameters for the retry middleware. Zero values
// are replaced with the defaults documented below when NewRetryMiddleware is called.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts after the first failure.
	// A value of 3 means the provider is called at most 4 times (1 original + 3 retries).
	// Default: 3.
	MaxRetries int

	// InitialBackoff is the wait duration before the first retry attempt.
	// Default: 1s.
	InitialBackoff time.Duration

	// MaxBackoff caps the computed backoff so it never exceeds this value.
	// Default: 30s.
	MaxBackoff time.Duration

	// BackoffFactor is the exponential growth multiplier applied to InitialBackoff
	// on successive retries (backoff = min(InitialBackoff * BackoffFactor^attempt, MaxBackoff)).
	// Default: 2.0.
	BackoffFactor float64

	// JitterFraction adds random noise to the computed backoff in the range
	// [0, JitterFraction * backoff] to avoid thundering-herd problems.
	// Default: 0.1 (10% jitter).
	JitterFraction float64

	// RetryableFunc returns true when an error should trigger a retry.
	// The default implementation retries on HTTP status codes 429, 500, 502, 503, and 529
	// by performing a string match on the error message.
	RetryableFunc func(error) bool
}

// defaultRetryableFunc returns true for transient HTTP errors (429, 500, 502, 503, 529).
// It inspects the error string because provider errors carry status codes as text.
func defaultRetryableFunc(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	for _, code := range []string{"429", "500", "502", "503", "529"} {
		if strings.Contains(msg, code) {
			return true
		}
	}

	return false
}

// applyRetryDefaults fills in zero-valued fields in config with sensible defaults.
func applyRetryDefaults(config *RetryConfig) {
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	if config.InitialBackoff == 0 {
		config.InitialBackoff = time.Second
	}

	if config.MaxBackoff == 0 {
		config.MaxBackoff = 30 * time.Second
	}

	if config.BackoffFactor == 0 {
		config.BackoffFactor = 2.0
	}

	if config.JitterFraction == 0 {
		config.JitterFraction = 0.1
	}

	if config.RetryableFunc == nil {
		config.RetryableFunc = defaultRetryableFunc
	}
}

// computeBackoff returns the backoff duration for the given attempt (0-indexed).
// backoff = min(InitialBackoff * BackoffFactor^attempt, MaxBackoff) + jitter
func computeBackoff(config RetryConfig, attempt int) time.Duration {
	base := float64(config.InitialBackoff) * math.Pow(config.BackoffFactor, float64(attempt))
	if base > float64(config.MaxBackoff) {
		base = float64(config.MaxBackoff)
	}

	jitter := base * config.JitterFraction * rand.Float64() //nolint:gosec // non-cryptographic jitter is intentional
	return time.Duration(base + jitter)
}

// NewRetryMiddleware constructs a MiddlewareConfig that retries failed send
// requests according to the supplied RetryConfig. Zero-valued fields in config
// are replaced with safe defaults (see RetryConfig documentation).
//
// The Stream field of the returned MiddlewareConfig is nil; streaming requests
// bypass this middleware because mid-stream errors cannot be transparently retried.
//
// On exhaustion the returned error wraps both [ErrRetryExhausted] and the last
// provider error, allowing callers to unwrap either.
func NewRetryMiddleware(config RetryConfig) client.MiddlewareConfig {
	applyRetryDefaults(&config)

	sendMiddleware := client.Middleware(func(next client.SendFunc) client.SendFunc {
		return func(ctx context.Context, request ai.ChatRequest) (*ai.ChatResponse, error) {
			var lastErr error

			for attempt := 0; attempt <= config.MaxRetries; attempt++ {
				if attempt > 0 {
					// Respect context cancellation between retries.
					backoff := computeBackoff(config, attempt-1)
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(backoff):
					}
				}

				response, err := next(ctx, request)
				if err == nil {
					return response, nil
				}

				lastErr = err

				if !config.RetryableFunc(err) {
					// Non-retryable error — propagate immediately.
					return nil, err
				}
			}

			return nil, fmt.Errorf("%w after %d retries: %w", ErrRetryExhausted, config.MaxRetries, lastErr)
		}
	})

	return client.MiddlewareConfig{
		Send:   sendMiddleware,
		Stream: nil, // Streaming cannot be transparently retried.
	}
}
