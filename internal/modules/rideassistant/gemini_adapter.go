// README: Adapter bridging ai.GeminiProvider to rideassistant.Planner interface.
package rideassistant

import (
	"context"
	"fmt"
	"time"

	"ark/internal/ai"
)

// GeminiAdapter implements the Planner interface using the AI provider directly.
type GeminiAdapter struct {
	provider *ai.GeminiProvider
	loc      *time.Location
}

// NewGeminiAdapter creates an adapter that bridges the AI provider to the ride assistant.
func NewGeminiAdapter(provider *ai.GeminiProvider) *GeminiAdapter {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		loc = time.UTC
	}
	return &GeminiAdapter{provider: provider, loc: loc}
}

// Parse calls the AI provider and converts IntentResult into ParserResponse.
func (a *GeminiAdapter) Parse(ctx context.Context, req ParserRequest) (*ParserResponse, error) {
	// Build context map for the AI provider.
	now := time.Now().In(a.loc)
	ctxMap := map[string]string{
		"current_time":      now.Format(time.RFC3339),
		"user_location":     req.SessionState["pickup_text"],
		"user_context_info": req.ContextInfo,
	}

	// Include session state as conversation context so AI can see what's already known.
	prompt := req.UserMessage
	if sessionCtx := buildSessionContext(req.SessionState); sessionCtx != "" {
		prompt = fmt.Sprintf("Conversation Context:\n%s\n\nUser Message: %s", sessionCtx, req.UserMessage)
	}

	intent, err := a.provider.ParseUserIntent(ctx, prompt, ctxMap)
	if err != nil {
		return nil, fmt.Errorf("gemini parse: %w", err)
	}

	return a.toParserResponse(intent), nil
}

// toParserResponse converts ai.IntentResult to rideassistant.ParserResponse.
func (a *GeminiAdapter) toParserResponse(ir *ai.IntentResult) *ParserResponse {
	resp := &ParserResponse{
		Intent: mapIntent(ir.Intent),
		Reply:  ir.Reply,
	}

	// Map start_location → pickup_text
	if ir.StartLocation != nil && *ir.StartLocation != "" && *ir.StartLocation != "Current Location" {
		resp.PickupText = ir.StartLocation
	}

	// Map destination → dropoff_text
	if ir.Destination != nil && *ir.Destination != "" {
		resp.DropoffText = ir.Destination
	}

	// Map iso_time → departure_at
	if ir.ISOTime != nil && *ir.ISOTime != "" {
		resp.DepartureAt = ir.ISOTime
	}

	// Build missing fields list from what the AI tells us.
	resp.MissingFields = a.inferMissingFields(ir)

	// Determine confirmation / booking readiness.
	if ir.Intent == "booking" && len(resp.MissingFields) == 0 {
		resp.NeedsConfirmation = true
	}
	if ir.Intent == "completed" {
		resp.ReadyToBook = true
	}

	return resp
}

// mapIntent normalizes the AI intent string to our internal status values.
func mapIntent(intent string) string {
	switch intent {
	case "booking":
		return "booking"
	case "clarification":
		return "clarification"
	case "chat":
		return "chat"
	case "cancel":
		return "cancel"
	case "completed":
		return "completed"
	default:
		return "chat"
	}
}

// inferMissingFields determines which booking fields are still unknown.
func (a *GeminiAdapter) inferMissingFields(ir *ai.IntentResult) []string {
	if ir.Intent != "booking" {
		return nil
	}

	var missing []string

	hasOrigin := ir.StartLocation != nil && *ir.StartLocation != "" && *ir.StartLocation != "Current Location"
	if !hasOrigin && (ir.NeedsOrigin != nil && *ir.NeedsOrigin) {
		missing = append(missing, "pickup")
	}

	if ir.Destination == nil || *ir.Destination == "" {
		missing = append(missing, "dropoff")
	}

	if ir.ISOTime == nil || *ir.ISOTime == "" {
		missing = append(missing, "departure_time")
	}

	return missing
}

// buildSessionContext formats the session state into a readable string for the AI.
func buildSessionContext(state map[string]string) string {
	if len(state) == 0 {
		return ""
	}
	var parts []string
	if v, ok := state["pickup_text"]; ok {
		parts = append(parts, fmt.Sprintf("Origin: %s", v))
	}
	if v, ok := state["dropoff_text"]; ok {
		parts = append(parts, fmt.Sprintf("Destination: %s", v))
	}
	if v, ok := state["departure_at"]; ok {
		parts = append(parts, fmt.Sprintf("Departure Time: %s", v))
	}
	if v, ok := state["stage"]; ok {
		parts = append(parts, fmt.Sprintf("Stage: %s", v))
	}
	if v, ok := state["pending_question"]; ok {
		parts = append(parts, fmt.Sprintf("Last Question Asked: %s", v))
	}
	if len(parts) == 0 {
		return ""
	}
	result := ""
	for _, p := range parts {
		result += "- " + p + "\n"
	}
	return result
}
