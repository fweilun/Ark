package ai

import (
	"context"
)

// LLMProvider defines the contract for interacting with AI models.
// This interface allows for swapping different AI providers (Gemini, OpenAI, etc.) in the future.
type LLMProvider interface {
	// ParseUserIntent analyzes the user's natural language input and extracts structured intent.
	// contextMap contains dynamic information like "current_time", "user_location", etc.
	ParseUserIntent(ctx context.Context, userMessage string, currentContext map[string]string) (*IntentResult, error)

	// PlanItinerary is a placeholder for V2 advanced route planning features.
	// It takes constraints and returns a suggested itinerary.
	PlanItinerary(ctx context.Context, constraints string) (string, error)
}
