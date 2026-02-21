// Package utils provides shared low-level helpers used throughout the aigo
// internals. It covers HTTP request helpers for both synchronous and
// streaming (SSE) communication with AI provider APIs, generic pointer and
// string utilities, and a simple elapsed-time timer.
//
// Key entry points: [DoPostSync] for synchronous JSON round-trips,
// [DoPostStream] together with [SSEScanner] for Server-Sent Events streaming,
// [Ptr] for converting values to pointers, and [Timer] for measuring latency.
package utils
