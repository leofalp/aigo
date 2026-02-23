package middleware

import "errors"

// ErrRetryExhausted is returned by the retry middleware when all retry attempts
// have been consumed without a successful response from the provider. The error
// is wrapped with the last underlying provider error so callers can use
// [errors.Is] / [errors.As] to inspect the root cause.
//
// Example:
//
//	if errors.Is(err, middleware.ErrRetryExhausted) {
//	    // all retries failed
//	}
var ErrRetryExhausted = errors.New("aigo: all retry attempts exhausted")
