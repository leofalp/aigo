// Package exa provides AIGO tool implementations backed by the Exa AI-native
// search API. It exposes three distinct capabilities: semantic web search via
// [NewExaSearchTool] and [NewExaSearchAdvancedTool], similarity search via
// [NewExaFindSimilarTool], and grounded question answering via
// [NewExaAnswerTool]. All tools require the EXA_API_KEY environment variable
// to be set before use.
package exa
