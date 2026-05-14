package aiusage

import "context"

// AIClient is the abstraction over any LLM provider (Gemini, OpenAI, etc.).
// All Gemini-specific implementation details remain inside gemini.go.
type AIClient interface {
	// ParseUserIntent analyses the user's natural-language message and returns
	// a structured IntentResult. contextMap supplies dynamic runtime values such
	// as "current_time", "user_location", and "user_context_info".
	ParseUserIntent(ctx context.Context, userMessage string, contextMap map[string]string) (*IntentResult, error)

	// PlanItinerary is the V2 hook for advanced multi-stop route planning.
	PlanItinerary(ctx context.Context, constraints string) (string, error)

	// Close releases any underlying network or SDK resources.
	Close()
}
