// Package overview provides execution lifecycle tracking for AI client sessions.
// It collects token usage, tool call statistics, cost breakdowns, and the full
// request/response history produced during a single execution run.
// The central type is [Overview]; use [OverviewFromContext] to obtain or create
// an instance bound to a [context.Context], and [Overview.CostSummary] to
// retrieve a detailed cost breakdown after execution completes.
package overview
