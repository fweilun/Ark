package rideassistant

import (
	"testing"

	"ark/internal/ai"
)

func TestToParserResponse_Booking(t *testing.T) {
	dest := "桃園機場"
	origin := "台北車站"
	isoTime := "2026-03-10T17:30:00+08:00"
	adapter := &GeminiAdapter{}

	ir := &ai.IntentResult{
		Intent:        "booking",
		Destination:   &dest,
		StartLocation: &origin,
		ISOTime:       &isoTime,
		Reply:         "確認從台北車站到桃園機場？",
	}

	resp := adapter.toParserResponse(ir)

	if resp.Intent != "booking" {
		t.Errorf("expected intent=booking, got %s", resp.Intent)
	}
	if resp.PickupText == nil || *resp.PickupText != "台北車站" {
		t.Error("expected pickup_text=台北車站")
	}
	if resp.DropoffText == nil || *resp.DropoffText != "桃園機場" {
		t.Error("expected dropoff_text=桃園機場")
	}
	if resp.DepartureAt == nil || *resp.DepartureAt != isoTime {
		t.Error("expected departure_at to be set")
	}
	if !resp.NeedsConfirmation {
		t.Error("expected needs_confirmation=true when all fields present")
	}
	if len(resp.MissingFields) != 0 {
		t.Errorf("expected no missing fields, got %v", resp.MissingFields)
	}
}

func TestToParserResponse_Clarification(t *testing.T) {
	adapter := &GeminiAdapter{}
	needsOrigin := true

	ir := &ai.IntentResult{
		Intent:      "booking",
		NeedsOrigin: &needsOrigin,
		Reply:       "請問您要從哪裡出發？",
	}

	resp := adapter.toParserResponse(ir)

	if resp.Intent != "booking" {
		t.Errorf("expected intent=booking, got %s", resp.Intent)
	}
	if len(resp.MissingFields) == 0 {
		t.Error("expected missing fields")
	}
}

func TestToParserResponse_Chat(t *testing.T) {
	adapter := &GeminiAdapter{}

	ir := &ai.IntentResult{
		Intent: "chat",
		Reply:  "我主要協助叫車。",
	}

	resp := adapter.toParserResponse(ir)

	if resp.Intent != "chat" {
		t.Errorf("expected intent=chat, got %s", resp.Intent)
	}
}

func TestToParserResponse_Completed(t *testing.T) {
	adapter := &GeminiAdapter{}

	ir := &ai.IntentResult{
		Intent: "completed",
		Reply:  "行程已確認！",
	}

	resp := adapter.toParserResponse(ir)

	if resp.Intent != "completed" {
		t.Errorf("expected intent=completed, got %s", resp.Intent)
	}
	if !resp.ReadyToBook {
		t.Error("expected ready_to_book=true for completed intent")
	}
}

func TestToParserResponse_CurrentLocation(t *testing.T) {
	adapter := &GeminiAdapter{}
	currentLoc := "Current Location"

	ir := &ai.IntentResult{
		Intent:        "booking",
		StartLocation: &currentLoc,
		Reply:         "請問出發地？",
	}

	resp := adapter.toParserResponse(ir)

	if resp.PickupText != nil {
		t.Error("expected pickup_text to be nil for 'Current Location'")
	}
}

func TestBuildSessionContext(t *testing.T) {
	state := map[string]string{
		"pickup_text":  "台北車站",
		"dropoff_text": "桃園機場",
		"stage":        "collecting",
	}
	result := buildSessionContext(state)
	if result == "" {
		t.Error("expected non-empty context")
	}
}

func TestMapIntent_Unknown(t *testing.T) {
	if mapIntent("unknown_intent") != "chat" {
		t.Error("unknown intents should map to chat")
	}
}
