package rideassistant

import (
	"context"
	"testing"
	"time"
)

// mockPlanner lets tests control AI responses.
type mockPlanner struct {
	response *ParserResponse
	err      error
}

func (m *mockPlanner) Parse(_ context.Context, _ ParserRequest) (*ParserResponse, error) {
	return m.response, m.err
}

func newTestService(planner Planner) *Service {
	return NewService(NewStore(), planner, nil)
}

func TestHandleMessage_Clarification(t *testing.T) {
	pickup := "台北車站"
	planner := &mockPlanner{response: &ParserResponse{
		Intent:        "booking",
		Reply:         "請問您要去哪裡？",
		PickupText:    &pickup,
		MissingFields: []string{"dropoff", "departure_time"},
	}}
	svc := newTestService(planner)

	resp, err := svc.HandleMessage(context.Background(), "user1", MessageRequest{
		Message: "我在台北車站",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "clarification" {
		t.Errorf("expected status=clarification, got %s", resp.Status)
	}
	if resp.Session == nil {
		t.Fatal("expected session in response")
	}
	if resp.Session.KnownFields["pickup_text"] != "台北車站" {
		t.Errorf("expected pickup_text=台北車站, got %s", resp.Session.KnownFields["pickup_text"])
	}
}

func TestHandleMessage_Confirmation(t *testing.T) {
	pickup := "台北車站"
	dropoff := "桃園機場"
	dep := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	planner := &mockPlanner{response: &ParserResponse{
		Intent:            "booking",
		Reply:             "確認從台北車站到桃園機場？",
		PickupText:        &pickup,
		DropoffText:       &dropoff,
		DepartureAt:       &dep,
		NeedsConfirmation: true,
	}}
	svc := newTestService(planner)

	resp, err := svc.HandleMessage(context.Background(), "user1", MessageRequest{Message: "book"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "confirmation" {
		t.Errorf("expected status=confirmation, got %s", resp.Status)
	}
}

func TestHandleMessage_Completed(t *testing.T) {
	pickup := "台北車站"
	dropoff := "桃園機場"
	dep := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	planner := &mockPlanner{response: &ParserResponse{
		Intent:      "booking",
		Reply:       "已幫您預約叫車。",
		PickupText:  &pickup,
		DropoffText: &dropoff,
		DepartureAt: &dep,
		ReadyToBook: true,
	}}
	svc := newTestService(planner)

	resp, err := svc.HandleMessage(context.Background(), "user1", MessageRequest{Message: "confirm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "completed" {
		t.Errorf("expected status=completed, got %s", resp.Status)
	}
	if resp.Booking == nil {
		t.Fatal("expected booking result")
	}
}

func TestHandleMessage_Cancel(t *testing.T) {
	planner := &mockPlanner{response: &ParserResponse{
		Intent: "cancel",
		Reply:  "已取消叫車。",
	}}
	svc := newTestService(planner)

	// First create a session.
	_, _ = svc.HandleMessage(context.Background(), "user1", MessageRequest{Message: "hello"})
	// Then cancel.
	planner.response = &ParserResponse{Intent: "cancel", Reply: "已取消叫車。"}
	resp, err := svc.HandleMessage(context.Background(), "user1", MessageRequest{Message: "cancel"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "cancelled" {
		t.Errorf("expected status=cancelled, got %s", resp.Status)
	}
}

func TestHandleMessage_Chat(t *testing.T) {
	planner := &mockPlanner{response: &ParserResponse{
		Intent: "chat",
		Reply:  "我主要可以協助您叫車。",
	}}
	svc := newTestService(planner)

	resp, err := svc.HandleMessage(context.Background(), "user1", MessageRequest{Message: "what's the weather?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "chat" {
		t.Errorf("expected status=chat, got %s", resp.Status)
	}
}

func TestHandleMessage_PastDeparture(t *testing.T) {
	pickup := "台北車站"
	dropoff := "桃園機場"
	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	planner := &mockPlanner{response: &ParserResponse{
		Intent:      "booking",
		Reply:       "已幫您預約叫車。",
		PickupText:  &pickup,
		DropoffText: &dropoff,
		DepartureAt: &past,
		ReadyToBook: true,
	}}
	svc := newTestService(planner)

	resp, err := svc.HandleMessage(context.Background(), "user1", MessageRequest{Message: "confirm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "clarification" {
		t.Errorf("expected status=clarification (past departure rejected), got %s", resp.Status)
	}
}

func TestHandleMessage_SessionPersistence(t *testing.T) {
	pickup := "台北車站"
	planner := &mockPlanner{response: &ParserResponse{
		Intent:        "booking",
		Reply:         "去哪裡？",
		PickupText:    &pickup,
		MissingFields: []string{"dropoff"},
	}}
	svc := newTestService(planner)

	// First message sets pickup.
	resp1, _ := svc.HandleMessage(context.Background(), "user1", MessageRequest{Message: "台北車站"})
	sessionID := resp1.Session.ID

	// Second message — session should be reused.
	dropoff := "桃園機場"
	planner.response = &ParserResponse{
		Intent:      "booking",
		Reply:       "確認？",
		DropoffText: &dropoff,
	}
	resp2, _ := svc.HandleMessage(context.Background(), "user1", MessageRequest{Message: "桃園機場"})
	if resp2.Session.ID != sessionID {
		t.Error("expected same session to be reused")
	}
	if resp2.Session.KnownFields["pickup_text"] != "台北車站" {
		t.Error("expected pickup_text to persist across messages")
	}
	if resp2.Session.KnownFields["dropoff_text"] != "桃園機場" {
		t.Error("expected dropoff_text to be merged")
	}
}
