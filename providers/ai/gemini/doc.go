// Package gemini implements the [ai.Provider] and [ai.StreamProvider] interfaces
// for Google's Gemini generative language API.
//
// It handles request conversion from the generic [ai.ChatRequest] format to
// Gemini's generateContent wire format, response mapping back to [ai.ChatResponse],
// SSE-based streaming via the streamGenerateContent endpoint, capability detection
// per model, and token cost calculation using published Gemini pricing tiers.
//
// The primary entry point is [New], which reads GEMINI_API_KEY and
// GEMINI_API_BASE_URL from the environment. Use [GeminiProvider.WithAPIKey],
// [GeminiProvider.WithBaseURL], or [GeminiProvider.WithHttpClient] to configure
// the provider programmatically. Model metadata and pricing are exposed through
// [ModelRegistry], [GetModelInfo], [GetModelCost], and [CalculateCost].
package gemini
