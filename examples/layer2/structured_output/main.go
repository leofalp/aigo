package main

import (
	"context"
	"fmt"
	"log"

	"github.com/leofalp/aigo/core/client"
	"github.com/leofalp/aigo/internal/jsonschema"
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
	fmt.Println("This example demonstrates:")
	fmt.Println("- Using WithOutputSchema to guide LLM output structure")
	fmt.Println("- Parsing responses with ParseResponseAs[T]")
	fmt.Println("- Type-safe access to structured data")
	fmt.Println()

	ctx := context.Background()

	// Example 1: Product Review with structured output
	fmt.Println("--- Example 1: Product Review Analysis ---")
	exampleProductReview(ctx)

	fmt.Println("\n--- Example 2: Sentiment Analysis ---")
	exampleSentimentAnalysis(ctx)

	fmt.Println("\n--- Example 3: Simple Primitive Types ---")
	examplePrimitiveTypes(ctx)

	fmt.Println("\n--- Example 4: Without Schema (String Response) ---")
	exampleWithoutSchema(ctx)
}

func exampleProductReview(ctx context.Context) {
	// Create client
	c, err := client.NewClient(
		openai.NewOpenAIProvider(),
		client.WithMemory(inmemory.New()),
		client.WithSystemPrompt("You are a product review analyzer. Analyze the given text and extract structured review information."),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Sample review text
	reviewText := `I recently bought the XPhone 15 Pro and I'm impressed!
	The camera quality is outstanding and the battery lasts all day.
	The design is sleek and premium. However, it's quite expensive and
	doesn't come with a charger in the box. Overall, I'd recommend it
	to anyone looking for a flagship phone. Rating: 4/5`

	fmt.Printf("Review Text: %s\n\n", reviewText)

	// Send message with output schema for this specific request
	resp, err := c.SendMessage(ctx, fmt.Sprintf("Analyze this review: %s", reviewText),
		client.WithOutputSchema(jsonschema.GenerateJSONSchema[ProductReview]()),
	)
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	// Parse response into structured type
	review, err := client.ParseResponseAs[ProductReview](resp)
	if err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	// Type-safe access to structured data
	fmt.Printf("✓ Product: %s\n", review.ProductName)
	fmt.Printf("✓ Rating: %d/5\n", review.Rating)
	fmt.Printf("✓ Pros: %v\n", review.Pros)
	fmt.Printf("✓ Cons: %v\n", review.Cons)
	fmt.Printf("✓ Summary: %s\n", review.Summary)
	fmt.Printf("✓ Recommend: %v\n", review.Recommend)
}

func exampleSentimentAnalysis(ctx context.Context) {
	// Create client
	c, err := client.NewClient(
		openai.NewOpenAIProvider(),
		client.WithSystemPrompt("You are a sentiment analysis expert."),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	text := "I absolutely love this new feature! It's exactly what I needed and works flawlessly."

	fmt.Printf("Text: %s\n\n", text)

	// Send with sentiment analysis schema
	resp, err := c.SendMessage(ctx, fmt.Sprintf("Analyze the sentiment: %s", text),
		client.WithOutputSchema(jsonschema.GenerateJSONSchema[SentimentAnalysis]()),
	)
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	// Parse into sentiment analysis struct
	sentiment, err := client.ParseResponseAs[SentimentAnalysis](resp)
	if err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	fmt.Printf("✓ Sentiment: %s\n", sentiment.Sentiment)
	fmt.Printf("✓ Confidence: %.2f\n", sentiment.Confidence)
	fmt.Printf("✓ Keywords: %v\n", sentiment.Keywords)
}

func examplePrimitiveTypes(ctx context.Context) {
	c, err := client.NewClient(
		openai.NewOpenAIProvider(),
		client.WithSystemPrompt("You are a helpful assistant that provides concise answers."),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Example: Parse as boolean
	fmt.Println("Question: Is Paris the capital of France? (answer only true or false)")
	resp, err := c.SendMessage(ctx, "Is Paris the capital of France? Answer only with 'true' or 'false'")
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	boolResult, err := client.ParseResponseAs[bool](resp)
	if err != nil {
		log.Printf("Failed to parse as bool: %v", err)
		fmt.Printf("Raw response: %s\n", resp.Content)
	} else {
		fmt.Printf("✓ Boolean result: %v\n", boolResult)
	}

	// Example: Parse as integer
	fmt.Println("\nQuestion: How many continents are there? (answer with just a number)")
	resp, err = c.SendMessage(ctx, "How many continents are there? Answer with just the number.")
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	intResult, err := client.ParseResponseAs[int](resp)
	if err != nil {
		log.Printf("Failed to parse as int: %v", err)
		fmt.Printf("Raw response: %s\n", resp.Content)
	} else {
		fmt.Printf("✓ Integer result: %d\n", intResult)
	}
}

func exampleWithoutSchema(ctx context.Context) {
	// Client without output schema - works with plain strings
	c, err := client.NewClient(
		openai.NewOpenAIProvider(),
		client.WithSystemPrompt("You are a creative writer."),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	resp, err := c.SendMessage(ctx, "Write a one-sentence story about a robot.")
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	// Parse as string (always works)
	story, err := client.ParseResponseAs[string](resp)
	if err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	fmt.Printf("✓ Story: %s\n", story)

	// Or just use the raw content
	fmt.Printf("✓ Raw content: %s\n", resp.Content)
}
