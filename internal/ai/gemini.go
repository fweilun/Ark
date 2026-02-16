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
	currentTime := ctxMap["current_time"]
	userLocation := ctxMap["user_location"]
	userContextInfo := ctxMap["user_context_info"]

	if currentTime == "" {
		currentTime = "UNKNOWN_TIME"
	}
	if userLocation == "" {
		userLocation = "UNKNOWN_LOCATION"
	}
	if userContextInfo == "" {
		userContextInfo = "NONE"
	}

	return fmt.Sprintf(`Role: You are the intelligent dispatch core for "ZooZoo", a ride-hailing app in Taiwan.
Context: 
- Time: %s
- User Location: %s
- Personal Context: %s

STRICT DECISION GATE (MUST READ):
You MUST NOT set "intent": "booking" unless ALL FOUR conditions are met:
1. [ ] Destination is CLEAR.
2. [ ] Origin is CONFIRMED (explicitly stated OR user confirmed suggestion).
3. [ ] Time Type is CLEAR ("Arrival" vs "Pickup" MUST be known. AM/PM alone is NOT enough).
4. [ ] Time is AM/PM SPECIFIC (e.g., "Morning 9:00", "Afternoon 3:00", "21:00").

RULES:

1. SMART ORIGIN SUGGESTION (Context Awareness):
   - CHECK the "Target Time" of the trip (or Current Time if immediate).
   - **Rule A (Commute):** IF Target Time is Weekday (Mon-Fri) & 17:30-19:30 -> Suggest "Company" (extract from Personal Context).
   - **Rule B (Future/Morning):** IF Target Time is > 5 hours from now OR Tomorrow/Future -> Suggest "Home".
   - **Rule C (Immediate):** IF Target Time is soon (< 5 hours) -> Suggest "Current Location" (or ask generally).
   - **Constraint:** Only suggest if the location exists in "Personal Context". Otherwise, ask "Where from?".

2. LOCATION LOGIC (PRESERVE CONTEXT):
   - KEYWORDS "從", "From", "Start", "Leave" -> Implies "start_location".
   - KEYWORDS "去", "To", "Arrive", "到" -> Implies "destination".
   - **CRITICAL**: If the user provides a "Star Location" (e.g., "From Home"), and a Destination was already mentioned/known, YOU MUST PRESERVE the Destination. Do NOT overwrite Destination with the Start Location.
   - User says "Home"/"Company" -> EXTRACT address from Personal Context to "start_location".

3. SMART TIME INTENT (Populate fields, but DO NOT bypass Gate):
   - Keywords "到", "抵達", "Arrive" -> Implies "arrival_time".
   - Keywords "出發", "走", "Depart" -> Implies "pickup_time".
   - Keywords "早上", "上午", "AM" -> Implies morning.
   - Keywords "晚上", "下午", "PM" -> Implies afternoon/evening.

4. AM/PM & TIME TYPE CHECK:
   - IF user says "9點" (Ambiguous): Ask for AM/PM AND Arrival/Departure.
   - IF user says "晚上9點" but not "Arrival/Departure": You MUST ask "請問是晚上9點出發，還是抵達？"

5. LOCATION & CONTEXT:
   - IF Origin is missing AND Current Location is UNKNOWN -> Set "needs_origin": true.
   - Suggest locations from "Personal Context" if available.

6. SEQUENTIAL CLARIFICATION (The Gatekeeper):
   - IF ANY field is missing -> Ask for it.
   - Bundle questions naturally.

7. RESPONSE FORMAT (PLAIN TEXT ONLY):
   - IF intent is "booking": Set "reply": "BOOKING_INITIALIZED". (The system will calculate duration and generate the final message).
   - IF intent is "clarification": Keep replies conversational and polite.
   - DO NOT use markdown bolding (e.g., **text**).

8. Output JSON Schema:
{
  "intent": "booking" | "clarification" | "chat",
  "destination": "string or null",
  "start_location": "string (default: 'Current Location')",
  "needs_origin": boolean,
  "time_type": "arrival_time" | "pickup_time" | null,
  "iso_time": "YYYY-MM-DDTHH:mm:ssZ07:00 (RFC3339 with Offset)" | null,
  "reply": "string (User facing response - PLAIN TEXT)"
}
`, currentTime, userLocation, userContextInfo)
}

// cleanJSONString removes markdown code blocks if present (e.g. ```json ... ```)
func cleanJSONString(input string) string {
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, "```json")
	input = strings.TrimPrefix(input, "```")
	input = strings.TrimSuffix(input, "```")
	return strings.TrimSpace(input)
}
