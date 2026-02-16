package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiProvider implements LLMProvider using Google's Gemini models.
type GeminiProvider struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

// NewGeminiProvider initializes a new Gemini client.
// apiKey should be provided from environment variables.
func NewGeminiProvider(ctx context.Context, apiKey string) (*GeminiProvider, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	// Use Gemini 2.0 Flash for low latency and cost efficiency.
	model := client.GenerativeModel("gemini-2.0-flash")

	// Force JSON response for structured parsing.
	model.ResponseMIMEType = "application/json"

	// Set a reasonable temperature for creative but structured output.
	model.SetTemperature(0.4)

	return &GeminiProvider{
		client: client,
		model:  model,
	}, nil
}

// Close cleans up the Gemini client resources.
func (p *GeminiProvider) Close() {
	p.client.Close()
}

// ParseUserIntent analyzes user input to extract ride-hailing intent.
func (p *GeminiProvider) ParseUserIntent(ctx context.Context, userMessage string, currentContext map[string]string) (*IntentResult, error) {
	// Construct a powerful system prompt with context injection.
	systemPrompt := buildSystemPrompt(currentContext)

	// Combine system prompt and user message.
	// Note: While Gemini supports SystemInstruction, appending context directly to the prompt
	// is often more flexible for dynamic context injection per request.
	// We'll use a combined prompt approach here for clarity and context binding.

	fullPrompt := fmt.Sprintf("%s\n\nUser Message: %s", systemPrompt, userMessage)

	resp, err := p.model.GenerateContent(ctx, genai.Text(fullPrompt))
	if err != nil {
		return nil, fmt.Errorf("gemini generation error: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("no response candidates from Gemini")
	}

	// Extract text from the response parts.
	var responseText strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			responseText.WriteString(string(txt))
		}
	}

	// Clean up potential markdown formatting (though json mode should handle this, safety first).
	cleanJSON := cleanJSONString(responseText.String())

	var result IntentResult
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w. Raw: %s", err, cleanJSON)
	}

	return &result, nil
}

// PlanItinerary is a placeholder for V2.
func (p *GeminiProvider) PlanItinerary(ctx context.Context, constraints string) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

// buildSystemPrompt constructs the instructions for the AI.
func buildSystemPrompt(ctxMap map[string]string) string {
	var contextBuilder strings.Builder
	for k, v := range ctxMap {
		contextBuilder.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
	}

	return fmt.Sprintf(`You are a helpful and efficient Taiwanese Ride Dispatcher (ZooZoo AI).
Your goal is to extract ride booking intents from user messages.
You MUST output strictly valid JSON. Do not output markdown code blocks.

Context:
%s

Output Schema (JSON):
{
  "intent": "booking" | "chat" | "route_planning",
  "destination": "extracted destination string or null",
  "time_type": "arrival_time" | "pickup_time" | "immediate",
  "iso_time": "YYYY-MM-DDTHH:mm:ss" (absolute time calculated from user input relative to context time) or null,
  "passenger_note": "any extra requests" or null,
  "reply": "Polite response in Traditional Chinese (Taiwan)"
}

Rules:
1. If the user wants a ride now, set "time_type" to "immediate".
2. If the user specifies a time (e.g., "tomorrow 9am"), calculate the "iso_time" based on the Current Time in context.
3. Be smart about "home", "work", or common landmarks.
4. If the intent is unclear, set "intent" to "chat" and ask for clarification in "reply".
5. DO NOT calculate prices or duration. Routing is done by Google Maps Service.
`, contextBuilder.String())
}

// cleanJSONString removes markdown code blocks if present (e.g. ```json ... ```)
func cleanJSONString(input string) string {
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, "```json")
	input = strings.TrimPrefix(input, "```")
	input = strings.TrimSuffix(input, "```")
	return strings.TrimSpace(input)
}
