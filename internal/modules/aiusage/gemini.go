package aiusage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const geminiModelName = "gemini-2.0-flash"

// geminiClient is the private Gemini implementation of AIClient.
// All Gemini-specific details are contained here.
type geminiClient struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

// NewGeminiClient creates a ready-to-use AIClient backed by Gemini.
// The caller must call Close() when the client is no longer needed.
func NewGeminiClient(ctx context.Context, apiKey string) (AIClient, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("gemini: missing api key")
	}
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("gemini: create client: %w", err)
	}
	model := client.GenerativeModel(geminiModelName)
	model.ResponseMIMEType = "application/json"
	model.SetTemperature(0.4)
	return &geminiClient{client: client, model: model}, nil
}

// Close releases Gemini SDK resources.
func (g *geminiClient) Close() {
	g.client.Close()
}

// ParseUserIntent sends the user message to Gemini and parses the structured JSON response.
func (g *geminiClient) ParseUserIntent(ctx context.Context, userMessage string, contextMap map[string]string) (*IntentResult, error) {
	systemPrompt := buildSystemPrompt(contextMap)
	fullPrompt := fmt.Sprintf("%s\n\nUser Message: %s", systemPrompt, userMessage)

	resp, err := g.model.GenerateContent(ctx, genai.Text(fullPrompt))
	if err != nil {
		return nil, fmt.Errorf("gemini: generate content: %w", err)
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("gemini: API returned no candidates")
	}

	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			sb.WriteString(string(txt))
		}
	}

	cleanJSON := cleanJSONString(sb.String())
	var result IntentResult
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return nil, fmt.Errorf("gemini: parse JSON response: %w. Raw: %s", err, cleanJSON)
	}
	return &result, nil
}

// PlanItinerary is reserved for V2 advanced itinerary planning.
func (g *geminiClient) PlanItinerary(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("PlanItinerary: not implemented yet")
}

// newGeminiModel is a lower-level helper for the Service.Chat plain-text path.
func newGeminiModel(ctx context.Context, apiKey string) (*genai.Client, *genai.GenerativeModel, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, nil, fmt.Errorf("gemini: missing api key")
	}
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, nil, fmt.Errorf("gemini: create client: %w", err)
	}
	return client, client.GenerativeModel(geminiModelName), nil
}

// generateText sends a plain-text message to Gemini and returns the reply.
func generateText(ctx context.Context, model *genai.GenerativeModel, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("gemini: empty message")
	}
	resp, err := model.GenerateContent(ctx, genai.Text(message))
	if err != nil {
		return "", fmt.Errorf("gemini: generate content: %w", err)
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("gemini: API returned empty candidates")
	}
	var textParts []string
	for _, part := range resp.Candidates[0].Content.Parts {
		txt, ok := part.(genai.Text)
		if !ok || strings.TrimSpace(string(txt)) == "" {
			continue
		}
		textParts = append(textParts, string(txt))
	}
	if len(textParts) == 0 {
		return "", fmt.Errorf("gemini: API returned empty text parts")
	}
	return strings.Join(textParts, "\n"), nil
}

// buildSystemPrompt constructs the AI persona and rule-set.
// contextMap keys: "current_time", "user_location", "user_context_info".
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
- Current System Time: %s
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

5. PAST TIME AUTO-CORRECTION (CRITICAL):
   - Compare the user's requested time with "Current System Time".
   - IF user requests a time that is EARLIER than "Current System Time" TODAY:
     - The time has already passed. Set "intent": "clarification".
     - Calculate tomorrow's date from the context.
     - Set "reply" to: "由於現在時間已晚，請問您是指 **明天 (M/DD)** [TIME_PERIOD][HH:MM] 抵達 [DESTINATION] 嗎？"
       - Replace M/DD with tomorrow's month/day (e.g., 2/17).
       - Replace [TIME_PERIOD] with 早上/下午/晚上 as appropriate.
       - Replace [HH:MM] with the requested time.
       - Replace [DESTINATION] with the destination.
     - Set "iso_time" to the NEXT DAY's datetime in RFC3339, NOT today's.
   - IF user confirms the "Tomorrow" suggestion, proceed normally with the corrected date.

6. LOCATION & CONTEXT:
   - IF Origin is missing AND Current Location is UNKNOWN -> Set "needs_origin": true.
   - Suggest locations from "Personal Context" if available.

7. SEARCH INTENT (V2):
   - IF user mentions a secondary task (e.g., "買花", "get coffee"):
     - **ORIGIN PRECONDITION (MANDATORY):** IF the origin/start_location is NOT yet confirmed in context:
       - Set "intent": "clarification", "needs_search": false.
       - Set "reply" to NATURALLY ask for origin FIRST.
       - NEVER set "needs_search": true until origin is confirmed.
     - ELSE (origin is known):
       - Set "needs_search": true.
       - Set "search_category": Translate to SPECIFIC PRECISE TERMS (English preferred for Places API).
         - E.g., "買花" -> "florist", "買咖啡" -> "coffee shop".
       - Set "search_keywords": Any POSITIVE refinement the user specifies.
       - Set "exclude_keywords": Terms the user explicitly wants to avoid.
         - **FLORIST DEFAULT:** When search_category is "florist", automatically set:
           "exclude_keywords": ["乾燥花", "永生花", "人造花", "香皂花", "塑膠花"]
         - If the user explicitly requests one of the above, REMOVE it from exclude_keywords.

8. INTERMEDIATE STOP SELECTION & STATE PRESERVATION (CRITICAL):
   - IF user selects an option from a provided list (e.g., "好", "第一個", "就那間"):
     - Set "intent": "booking", "intermediate_stop": the selected place name, "needs_search": false.
     - **MANDATORY FIELD CARRY-FORWARD:** Preserve destination, start_location, iso_time, time_type from context.
     - NEVER reset iso_time to null on a confirmation turn.

9. STRICT BOOKING GATES (CRITICAL):
   - BEFORE setting "intent": "booking", YOU MUST HAVE:
     1. SPECIFIC DESTINATION ("HSR" is INVALID. Ask "Taipei HSR or Nangang HSR?").
     2. SPECIFIC TIME ("9:00" is AMBIGUOUS. Ask "Morning or Evening?").
     3. CONFIRMED ORIGIN.
   - If ANY are missing, set "intent": "clarification" and ASK.

10. SEQUENTIAL CLARIFICATION (The Gatekeeper):
   - IF ANY field is missing -> Ask for it. Bundle questions naturally.

11. RESPONSE FORMAT & ABSOLUTE CONTENT RULES:
   ⛔ ABSOLUTE BAN: The "reply" field MUST NEVER contain: SEARCHING, BOOKING_INITIALIZED, COMPLETED, CLARIFICATION, or ANY ALL-CAPS system token.
   ✅ Use natural, conversational Traditional Chinese (台灣繁體中文口語).
   - DO NOT use markdown bolding IN THE reply FIELD.

12. PASSENGER & PET DETECTION (Scan ALL conversation history):
   - "passenger_count": Trigger phrases: "我們X個人", "X位", "一行X人". Default: 1. PERSIST across turns.
   - "has_pet": Set true if ANY mention of pet. Trigger: "帶狗", "帶貓", "寵物", "pet". PERSIST: once true, never reset to false.

13. UPSELL RESPONSE & COMPLETED STATE (CRITICAL):
   - IF conversation history shows ZooZoo already asked an upsell question AND user is responding:
     - Set "intent": "completed".
     - User DECLINES (e.g., "不用"): Set "selected_upgrade": "".
     - User ACCEPTS (e.g., "要豪車"): Set "selected_upgrade": the car name.
     - NEVER set intent to "booking" or "clarification" on a completed upsell turn.

14. Output JSON Schema:
{
  "intent": "booking" | "clarification" | "chat" | "completed",
  "destination": "string or null",
  "start_location": "string (default: 'Current Location')",
  "needs_origin": boolean,
  "needs_search": boolean,
  "search_category": "string or null",
  "search_keywords": "string or null",
  "exclude_keywords": ["string"],
  "intermediate_stop": "string or null",
  "time_type": "arrival_time" | "pickup_time" | null,
  "iso_time": "YYYY-MM-DDTHH:mm:ssZ07:00 (RFC3339 with Offset)" | null,
  "passenger_count": integer (default 1),
  "has_pet": boolean (default false),
  "selected_upgrade": "string (car type chosen by user, empty = declined)",
  "reply": "string (User facing response)"
}
`, currentTime, userLocation, userContextInfo)
}

// cleanJSONString removes markdown code fences that some models emit despite JSON mode.
func cleanJSONString(input string) string {
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, "```json")
	input = strings.TrimPrefix(input, "```")
	input = strings.TrimSuffix(input, "```")
	return strings.TrimSpace(input)
}
