package react

import (
	"aigo/core/client"
	"aigo/providers/ai"
	"aigo/providers/memory"
	"aigo/providers/observability"
	"aigo/providers/tool"
	"context"
	"fmt"
	"time"
)

// Config holds configuration for the ReAct pattern execution.
type Config struct {
	MaxIterations int  // Maximum number of tool execution iterations (default: 10)
	StopOnError   bool // Whether to stop on tool execution errors (default: true)
}

// DefaultConfig returns sensible defaults for ReAct pattern.
func DefaultConfig() Config {
	return Config{
		MaxIterations: 10,
		StopOnError:   true,
	}
}

// ReactClient wraps a base client and adds ReAct pattern behavior:
// automatic tool execution loop with reasoning.
type ReactClient[T any] struct {
	client      *client.Client[T]
	memory      memory.Provider
	toolCatalog map[string]tool.GenericTool
	config      Config
}

// NewReactClient creates a new ReAct pattern client that wraps a base client.
// The base client should be configured with tools and observer.
// Memory and tool catalog are required for the ReAct pattern to work.
func NewReactClient[T any](
	baseClient *client.Client[T],
	memoryProvider memory.Provider,
	toolCatalog map[string]tool.GenericTool,
	cfg Config,
) *ReactClient[T] {
	return &ReactClient[T]{
		client:      baseClient,
		memory:      memoryProvider,
		toolCatalog: toolCatalog,
		config:      cfg,
	}
}

// Execute runs the ReAct pattern loop:
// 1. Send user prompt to LLM
// 2. If LLM requests tool calls, execute them and add results to memory
// 3. Repeat until LLM provides final answer or max iterations reached
//
// Returns the final response from the LLM after the reasoning loop completes.
func (r *ReactClient[T]) Execute(ctx context.Context, prompt string) (*ai.ChatResponse, error) {
	observer := observability.ObserverFromContext(ctx)
	var span observability.Span

	// Start top-level ReAct span
	if observer != nil {
		ctx, span = observer.StartSpan(ctx, "react.execute",
			observability.String("pattern", "react"),
			observability.String("prompt", observability.TruncateStringDefault(prompt)),
			observability.Int("max_iterations", r.config.MaxIterations),
		)
		defer span.End()

		ctx = observability.ContextWithSpan(ctx, span)
		ctx = observability.ContextWithObserver(ctx, observer)

		observer.Info(ctx, "Starting ReAct pattern execution",
			observability.Int("max_iterations", r.config.MaxIterations),
			observability.Int("tools_available", len(r.toolCatalog)),
		)
	}

	startTime := time.Now()
	iteration := 0
	var response *ai.ChatResponse
	var err error

	// Main ReAct loop
	for iteration < r.config.MaxIterations {
		iteration++

		if observer != nil {
			observer.Info(ctx, "ReAct iteration starting",
				observability.Int("iteration", iteration),
				observability.Int("max_iterations", r.config.MaxIterations),
			)
		}

		// Step 1: Send message to LLM (first iteration uses prompt, subsequent use empty string)
		iterationStart := time.Now()
		var message string
		if iteration == 1 {
			message = prompt
		} else {
			// Empty message allows LLM to process tool results from memory
			message = ""
		}

		response, err = r.client.SendMessage(ctx, message)
		iterationDuration := time.Since(iterationStart)

		if err != nil {
			if observer != nil {
				span.RecordError(err)
				span.SetStatus(observability.StatusError, "ReAct iteration failed")
				observer.Error(ctx, "ReAct iteration failed",
					observability.Error(err),
					observability.Int("iteration", iteration),
					observability.Duration("iteration_duration", iterationDuration),
				)
			}
			return nil, fmt.Errorf("iteration %d failed: %w", iteration, err)
		}

		// Step 2: Check if we're done (no tool calls = final answer)
		if len(response.ToolCalls) == 0 {
			totalDuration := time.Since(startTime)

			if observer != nil {
				span.SetStatus(observability.StatusOK, "ReAct completed successfully")
				observer.Info(ctx, "ReAct pattern completed - no tool calls, final answer received",
					observability.Int("total_iterations", iteration),
					observability.Duration("total_duration", totalDuration),
					observability.String("finish_reason", response.FinishReason),
					observability.Bool("has_content", response.Content != ""),
				)

				// Record metrics
				observer.Counter("react.executions.total").Add(ctx, 1,
					observability.String("status", "success"),
				)
				observer.Histogram("react.iterations.count").Record(ctx, float64(iteration))
				observer.Histogram("react.duration.seconds").Record(ctx, totalDuration.Seconds())
			}

			return response, nil
		}

		// Step 3: Execute tool calls
		if observer != nil {
			observer.Info(ctx, "LLM requested tool calls - executing tools",
				observability.Int("iteration", iteration),
				observability.Int("tool_calls_count", len(response.ToolCalls)),
				observability.String("finish_reason", response.FinishReason),
			)

			// Log each tool call name
			for i, tc := range response.ToolCalls {
				observer.Debug(ctx, "Tool call details",
					observability.Int("tool_index", i),
					observability.String("tool_name", tc.Function.Name),
					observability.String("tool_type", tc.Type),
				)
			}
		}

		// Add assistant message to memory (with tool calls)
		r.memory.AppendMessage(ctx, &ai.Message{
			Role:    ai.RoleAssistant,
			Content: response.Content,
		})

		toolsExecuted := 0
		for _, toolCall := range response.ToolCalls {
			err := r.executeToolCall(ctx, observer, toolCall)
			if err != nil {
				if r.config.StopOnError {
					if observer != nil {
						span.RecordError(err)
						span.SetStatus(observability.StatusError, "Tool execution failed")
						observer.Error(ctx, "Tool execution failed, stopping ReAct loop",
							observability.Error(err),
							observability.String("tool_name", toolCall.Function.Name),
							observability.Int("iteration", iteration),
						)
					}
					return nil, fmt.Errorf("tool execution failed at iteration %d: %w", iteration, err)
				}

				// Continue with error message in memory
				if observer != nil {
					observer.Warn(ctx, "Tool execution failed, continuing",
						observability.Error(err),
						observability.String("tool_name", toolCall.Function.Name),
					)
				}
			} else {
				toolsExecuted++
			}
		}

		if observer != nil {
			observer.Info(ctx, "ReAct iteration completed - continuing to next iteration",
				observability.Int("iteration", iteration),
				observability.Int("tools_executed", toolsExecuted),
				observability.Int("tools_failed", len(response.ToolCalls)-toolsExecuted),
				observability.Duration("iteration_duration", iterationDuration),
			)

			// Record iteration metrics
			observer.Counter("react.iterations.total").Add(ctx, 1)
			observer.Counter("react.tools_executed.total").Add(ctx, int64(toolsExecuted))
		}
	}

	// Max iterations reached without final answer
	totalDuration := time.Since(startTime)

	if observer != nil {
		span.SetStatus(observability.StatusError, "Max iterations reached")
		observer.Warn(ctx, "ReAct pattern reached max iterations without final answer",
			observability.Int("max_iterations", r.config.MaxIterations),
			observability.Duration("total_duration", totalDuration),
		)

		observer.Counter("react.executions.total").Add(ctx, 1,
			observability.String("status", "max_iterations_reached"),
		)
	}

	return response, fmt.Errorf("reached maximum iterations (%d) without final answer", r.config.MaxIterations)
}

// executeToolCall executes a single tool call and adds the result to memory.
func (r *ReactClient[T]) executeToolCall(ctx context.Context, observer observability.Provider, toolCall ai.ToolCall) error {
	var span observability.Span

	if observer != nil {
		ctx, span = observer.StartSpan(ctx, "react.execute_tool",
			observability.String("tool_name", toolCall.Function.Name),
			observability.String("arguments", observability.TruncateString(toolCall.Function.Arguments, 500)),
		)
		defer span.End()

		observer.Info(ctx, "Executing tool",
			observability.String("tool_name", toolCall.Function.Name),
			observability.String("tool_type", toolCall.Type),
			observability.String("arguments", observability.TruncateString(toolCall.Function.Arguments, 200)),
		)
	}

	start := time.Now()

	// Look up tool in catalog
	toolInstance, exists := r.toolCatalog[toolCall.Function.Name]
	if !exists {
		err := fmt.Errorf("tool '%s' not found in catalog", toolCall.Function.Name)
		if observer != nil {
			span.RecordError(err)
			span.SetStatus(observability.StatusError, "Tool not found")
			observer.Error(ctx, "Tool not found in catalog",
				observability.String("tool_name", toolCall.Function.Name),
			)
		}
		return err
	}

	// Execute tool
	result, err := toolInstance.Call(ctx, toolCall.Function.Arguments)
	duration := time.Since(start)

	if err != nil {
		if observer != nil {
			span.RecordError(err)
			span.SetStatus(observability.StatusError, "Tool execution error")
			observer.Error(ctx, "Tool execution failed",
				observability.Error(err),
				observability.String("tool_name", toolCall.Function.Name),
				observability.Duration("duration", duration),
			)
		}

		// Add error result to memory
		errorMsg := fmt.Sprintf(`{"error": "%s"}`, err.Error())
		r.memory.AppendMessage(ctx, &ai.Message{
			Role:    ai.RoleTool,
			Content: errorMsg,
		})

		return err
	}

	// Add successful result to memory
	r.memory.AppendMessage(ctx, &ai.Message{
		Role:    ai.RoleTool,
		Content: result,
	})

	if observer != nil {
		span.SetStatus(observability.StatusOK, "Tool executed successfully")
		observer.Info(ctx, "Tool executed successfully - result added to memory",
			observability.String("tool_name", toolCall.Function.Name),
			observability.Duration("duration", duration),
			observability.Int("result_length", len(result)),
			observability.String("result_preview", observability.TruncateString(result, 100)),
		)
	}

	return nil
}
