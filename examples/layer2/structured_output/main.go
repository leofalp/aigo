package main

import (
	"context"
	"fmt"
	"log"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/internal/jsonschema"
	"github.com/leofalp/aigo/internal/utils"
	"github.com/leofalp/aigo/providers/ai/openai"
	"github.com/leofalp/aigo/providers/memory/inmemory"

	_ "github.com/joho/godotenv/autoload"
)

// ProductReview represents a structured product review response
type ProductReview struct {
	ProductName string   `json:"product_name" jsonschema:"required,description=Name of the product being reviewed"`
	Rating      int      `json:"rating" jsonschema:"required,description=Rating from 1 to 5"`
	Pros        []string `json:"pros" jsonschema:"description=List of positive aspects"`
	Cons        []string `json:"cons" jsonschema:"description=List of negative aspects"`
	Summary     string   `json:"summary" jsonschema:"required,description=Brief summary of the review"`
	Recommend   bool     `json:"recommend" jsonschema:"required,description=Whether to recommend the product"`
}

// SentimentAnalysis represents sentiment analysis result
type SentimentAnalysis struct {
	Sentiment  string   `json:"sentiment" jsonschema:"required,enum=positive,enum=negative,enum=neutral"`
	Confidence float64  `json:"confidence" jsonschema:"required,description=Confidence score between 0 and 1"`
	Keywords   []string `json:"keywords,omitempty" jsonschema:"description=Key words or phrases identified"`
}

func main() {
	fmt.Println("=== Structured Output Examples ===")
	fmt.Println()
	fmt.Println("This example shows TWO approaches for structured output:")
	fmt.Println("1. AUTOMATIC (StructuredClient) - Recommended ⭐")
	fmt.Println("2. MANUAL (WithOutputSchema + ParseStringAs)")

	ctx := context.Background()

	// Approach 1: Automatic with StructuredClient (Recommended)
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  APPROACH 1: StructuredClient (Automatic) - RECOMMENDED ⭐   ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	exampleAutomatic(ctx)

	fmt.Println()
	fmt.Println()

	// Approach 2: Manual with WithOutputSchema
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  APPROACH 2: Manual (WithOutputSchema + ParseStringAs)       ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	exampleManual(ctx)
}

// exampleAutomatic demonstrates the recommended approach using StructuredClient
func exampleAutomatic(ctx context.Context) {
	fmt.Println("--- Product Review Analysis (Automatic) ---")
	fmt.Println()

	// Step 1: Create base client
	baseClient, err := client.New(
		openai.New(),
		client.WithMemory(inmemory.New()),
		client.WithSystemPrompt("You are a product review analyzer."),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Step 2: Wrap in StructuredClient - TYPE SPECIFIED ONCE!
	reviewClient := client.FromBaseClient[ProductReview](baseClient)

	reviewText := `I recently bought the XPhone 15 Pro and I'm impressed!
	The camera quality is outstanding and the battery lasts all day.
	The design is sleek and premium. However, it's quite expensive and
	doesn't come with a charger in the box. Overall, I'd recommend it
	to anyone looking for a flagship phone. Rating: 4/5`

	fmt.Printf("📝 Review: %s\n\n", reviewText)

	// Step 3: Send message - schema applied & response parsed automatically!
	resp, err := reviewClient.SendMessage(ctx, fmt.Sprintf("Analyze this review: %s", reviewText))
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	// Step 4: Access typed data directly - NO MANUAL PARSING!
	fmt.Println("✅ Parsed Results:")
	fmt.Printf("  Product:   %s\n", resp.Data.ProductName)
	fmt.Printf("  Rating:    %d/5\n", resp.Data.Rating)
	fmt.Printf("  Pros:      %v\n", resp.Data.Pros)
	fmt.Printf("  Cons:      %v\n", resp.Data.Cons)
	fmt.Printf("  Summary:   %s\n", resp.Data.Summary)
	fmt.Printf("  Recommend: %v\n", resp.Data.Recommend)

	// Access raw metadata
	fmt.Printf("\n📊 Metadata:\n")
	fmt.Printf("  Tokens: %d\n", resp.Usage.TotalTokens)
	fmt.Printf("  Model:  %s\n", resp.Model)

	fmt.Println("\n✨ Benefits:")
	fmt.Println("  ✓ Type specified ONCE (at client creation)")
	fmt.Println("  ✓ Schema generated automatically")
	fmt.Println("  ✓ Response parsed automatically")
	fmt.Println("  ✓ Type-safe data access")
	fmt.Println("  ✓ Metadata still accessible")

	// Multi-turn conversation example
	fmt.Println("\n--- Multi-turn Conversation (Automatic) ---")
	fmt.Println()

	type ConversationResponse struct {
		Answer     string `json:"answer" jsonschema:"required"`
		Confidence int    `json:"confidence" jsonschema:"required,description=Confidence 1-10"`
	}

	conversationClient := client.FromBaseClient[ConversationResponse](baseClient)

	resp1, _ := conversationClient.SendMessage(ctx, "What is the capital of France?")
	fmt.Printf("Q: What is the capital of France?\n")
	fmt.Printf("A: %s (confidence: %d/10)\n", resp1.Data.Answer, resp1.Data.Confidence)

	resp2, _ := conversationClient.SendMessage(ctx, "What's its population?")
	fmt.Printf("\nQ: What's its population?\n")
	fmt.Printf("A: %s (confidence: %d/10)\n", resp2.Data.Answer, resp2.Data.Confidence)

	fmt.Println("\n💡 All responses automatically follow the same schema!")
}

// exampleManual demonstrates the manual approach for comparison
func exampleManual(ctx context.Context) {
	fmt.Println("--- Sentiment Analysis (Manual) ---")
	fmt.Println()

	// Step 1: Create client
	baseClient, err := client.New(
		openai.New(),
		client.WithSystemPrompt("You are a sentiment analysis expert."),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	text := "I absolutely love this new feature! It's exactly what I needed."

	fmt.Printf("📝 Text: %s\n\n", text)

	// Step 2: Generate schema manually - TYPE SPECIFIED FIRST TIME
	schema := jsonschema.GenerateJSONSchema[SentimentAnalysis]()

	// Step 3: Send with schema
	resp, err := baseClient.SendMessage(ctx, fmt.Sprintf("Analyze the sentiment: %s", text),
		client.WithOutputSchema(schema), // Schema passed manually
	)
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	// Step 4: Parse manually - TYPE SPECIFIED SECOND TIME
	sentiment, err := utils.ParseStringAs[SentimentAnalysis](resp.Content)
	if err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	// Step 5: Access data
	fmt.Println("✅ Parsed Results:")
	fmt.Printf("  Sentiment:  %s\n", sentiment.Sentiment)
	fmt.Printf("  Confidence: %.2f\n", sentiment.Confidence)
	fmt.Printf("  Keywords:   %v\n", sentiment.Keywords)

	fmt.Printf("\n📊 Metadata:\n")
	fmt.Printf("  Tokens: %d\n", resp.Usage.TotalTokens)

	fmt.Println("\n⚠️  Downsides:")
	fmt.Println("  ✗ Type specified TWICE (schema + parsing)")
	fmt.Println("  ✗ Manual schema generation required")
	fmt.Println("  ✗ Manual parsing required")
	fmt.Println("  ✗ More boilerplate code")

	fmt.Println("\n💡 Use this approach only when you need:")
	fmt.Println("  • Different schemas for different requests")
	fmt.Println("  • Fine-grained control over schema generation")
	fmt.Println("  • To delay parsing until later")
}
