// Package jsonschema provides utilities for generating and representing JSON Schema
// structures from Go types using reflection.
//
// It supports structs, primitives, slices, maps, pointers, and recursive types.
// Recursive type references are automatically resolved using $ref and $defs to
// avoid infinite loops during schema generation.
//
// The main entry point is [GenerateJSONSchema], which derives a [Schema] from any
// Go type T at compile time without requiring a runtime value.
package jsonschema
