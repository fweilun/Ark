// README: Stub AI planner for development — returns static clarification responses.
// Replace with real Gemini integration in Phase 2.
package rideassistant

import "context"

// StubPlanner is a placeholder Planner that echoes messages and always asks for clarification.
type StubPlanner struct{}

// NewStubPlanner creates a stub planner.
func NewStubPlanner() *StubPlanner {
	return &StubPlanner{}
}

// Parse returns a static clarification response. This will be replaced by real AI integration.
func (p *StubPlanner) Parse(_ context.Context, req ParserRequest) (*ParserResponse, error) {
	// Check what's missing from session state to craft a basic reply.
	_, hasPickup := req.SessionState["pickup_text"]
	_, hasDropoff := req.SessionState["dropoff_text"]
	_, hasDeparture := req.SessionState["departure_at"]

	var missing []string
	if !hasPickup {
		missing = append(missing, "pickup")
	}
	if !hasDropoff {
		missing = append(missing, "dropoff")
	}
	if !hasDeparture {
		missing = append(missing, "departure_time")
	}

	reply := "收到您的訊息：「" + req.UserMessage + "」。請問您要從哪裡出發呢？"
	if len(missing) == 0 {
		reply = "資訊已齊全，請確認是否要幫您叫車？"
	}

	return &ParserResponse{
		Intent:            "booking",
		Reply:             reply,
		MissingFields:     missing,
		NeedsConfirmation: len(missing) == 0,
	}, nil
}
