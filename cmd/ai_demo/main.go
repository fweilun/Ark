package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"ark/internal/ai"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable not set")
	}

	ctx := context.Background()
	provider, err := ai.NewGeminiProvider(ctx, apiKey)
	if err != nil {
		log.Fatalf("Failed to initialize AI provider: %v", err)
	}
	defer provider.Close()

	// Simulated context
	currentContext := map[string]string{
		"current_time":  time.Now().Format(time.RFC3339),
		"user_location": "Taipei Main Station",
	}

	userMessage := "明天早上九點我要去信義微風"
	fmt.Printf("User: %s\n", userMessage)

	result, err := provider.ParseUserIntent(ctx, userMessage, currentContext)
	if err != nil {
		log.Fatalf("Error parsing intent: %v", err)
	}

	fmt.Printf("AI Reply: %s\n", result.Reply)
	fmt.Printf("Intent: %s\n", result.Intent)
	if result.Destination != nil {
		fmt.Printf("Destination: %s\n", *result.Destination)
	}
	if result.ISOTime != nil {
		fmt.Printf("Time: %s\n", *result.ISOTime)
	}
}
