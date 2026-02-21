// Package parse provides utilities for extracting and converting structured
// data from raw LLM text output. Because language models frequently wrap
// JSON in narrative prose, markdown code fences, or schema-style envelopes,
// this package applies a layered recovery strategy — candidate extraction,
// automatic JSON repair, and schema unwrapping — before falling back to a
// clear error.
//
// The main entry point is the generic [ParseStringAs] function, which handles
// both primitive types (string, bool, int, float) and complex types (structs,
// maps, slices) in a single, uniform API.
package parse
