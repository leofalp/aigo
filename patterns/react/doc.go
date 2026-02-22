// Package react implements the ReAct (Reasoning + Acting) agentic pattern on
// top of the core client. It drives an iterative tool-execution loop in which
// the LLM alternates between reasoning steps and tool calls until it produces
// a final answer, which is then parsed into a caller-defined Go type T.
//
// The main entry point is [New], which wraps a configured [client.Client] and
// returns a type-safe [ReAct] agent. Use [Execute] to run the loop for a given
// prompt. Behavior can be tuned with [WithMaxIterations] and [WithStopOnError].
package react
