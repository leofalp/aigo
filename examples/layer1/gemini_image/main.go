// Package main demonstrates image input (vision) and image output (generation)
// using the Gemini provider with AIGO's multimodal content types.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"

	"github.com/leofalp/aigo/providers/ai"
	"github.com/leofalp/aigo/providers/ai/gemini"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	provider := gemini.New()
	ctx := context.Background()

	// Example 1: Image input (vision) — describe a small test image
	// Creates a minimal 1x1 red PNG pixel for demonstration purposes
	redPixelPNG := createMinimalPNG()
	encodedImage := base64.StdEncoding.EncodeToString(redPixelPNG)

	fmt.Println("=== Example 1: Image Input (Vision) ===")
	visionResponse, err := provider.SendMessage(ctx, ai.ChatRequest{
		Model:        gemini.Model25Flash,
		SystemPrompt: "You are a helpful assistant that describes images. Be concise.",
		Messages: []ai.Message{
			{
				Role: ai.RoleUser,
				ContentParts: []ai.ContentPart{
					ai.NewTextPart("Describe this image in one sentence."),
					ai.NewImagePart("image/png", encodedImage),
				},
			},
		},
	})
	if err != nil {
		slog.Error("Vision request failed", "error", err)
		os.Exit(1)
	}
	fmt.Printf("Model response: %s\n\n", visionResponse.Content)

	// Example 2: Image generation — request the model to generate an image
	fmt.Println("=== Example 2: Image Generation ===")
	generationResponse, err := provider.SendMessage(ctx, ai.ChatRequest{
		Model: "gemini-2.5-flash-image",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Generate a simple icon of a blue circle on a white background."},
		},
		GenerationConfig: &ai.GenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
		},
	})
	if err != nil {
		slog.Error("Image generation request failed", "error", err)
		os.Exit(1)
	}

	if generationResponse.Content != "" {
		fmt.Printf("Text response: %s\n", generationResponse.Content)
	}

	if len(generationResponse.Images) > 0 {
		fmt.Printf("Generated %d image(s)\n", len(generationResponse.Images))
		for imageIndex, image := range generationResponse.Images {
			filename := fmt.Sprintf("generated_image_%d.png", imageIndex)
			imageBytes, decodeErr := base64.StdEncoding.DecodeString(image.Data)
			if decodeErr != nil {
				slog.Error("Failed to decode image", "error", decodeErr)
				continue
			}
			if writeErr := os.WriteFile(filename, imageBytes, 0644); writeErr != nil {
				slog.Error("Failed to write image file", "error", writeErr)
				continue
			}
			fmt.Printf("Saved image to %s (%s, %d bytes)\n", filename, image.MimeType, len(imageBytes))
		}
	} else {
		fmt.Println("No images were generated in the response.")
	}
}

// createMinimalPNG returns the raw bytes of a valid 1x1 red PNG image
// using Go's standard image/png encoder.
func createMinimalPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		panic(fmt.Sprintf("failed to encode PNG: %v", err))
	}
	return buffer.Bytes()
}
