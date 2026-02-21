// Package calculator provides a locally-executed arithmetic tool for use with
// the AIGO tool system. It supports the four basic operations: addition,
// subtraction, multiplication, and division over floating-point operands.
//
// The main entry point is [NewCalculatorTool], which returns a ready-to-use
// [tool.Tool] that can be registered with any AIGO client or tool catalog.
// The underlying computation function is also exported as [Calc] for cases
// where direct invocation is preferred over the tool wrapper.
package calculator
