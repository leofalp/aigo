// Package middleware provides built-in middleware implementations for the aigo
// client. Each middleware is constructed via a New* function that returns a
// [client.MiddlewareConfig] ready to be passed to [client.WithMiddleware].
//
// # Available Middleware
//
//   - [NewRetryMiddleware]: Retries failed provider calls with exponential backoff
//     and jitter. Useful for transient HTTP 429 / 5xx errors.
//
//   - [NewTimeoutMiddleware]: Adds a per-request deadline via context.WithTimeout,
//     ensuring that a stalled provider call does not block the caller indefinitely.
//
//   - [NewLoggingMiddleware]: Emits structured slog log entries before and after
//     every provider call, with three verbosity levels (Minimal, Standard, Verbose).
//
// # Usage
//
//	import (
//	    "log/slog"
//	    "time"
//
//	    "github.com/leofalp/aigo/core/client"
//	    "github.com/leofalp/aigo/core/client/middleware"
//	)
//
//	c, err := client.New(provider,
//	    client.WithMiddleware(
//	        middleware.NewTimeoutMiddleware(30*time.Second),
//	        middleware.NewRetryMiddleware(middleware.RetryConfig{MaxRetries: 3}),
//	        middleware.NewLoggingMiddleware(slog.Default(), middleware.LogLevelStandard),
//	    ),
//	)
//
// Middlewares execute outermost-first: the first entry in WithMiddleware is the
// outermost wrapper, meaning it runs first on the way in and last on the way out.
// In the example above, a request travels:
//
//	Timeout (first — outermost) → Retry → Logging → Provider
//
// and the response travels back in reverse:
//
//	Provider → Logging → Retry → Timeout (last — outermost)
package middleware
