// README: Models for the ride assistant module — session state machine, API shapes, AI parser contract.
package rideassistant

import "time"

// ---------------------------------------------------------------------------
// Session stages
// ---------------------------------------------------------------------------

const (
	StageCollecting = "collecting"
	StageConfirming = "confirming"
	StageCompleted  = "completed"
	StageCancelled  = "cancelled"
)

// ---------------------------------------------------------------------------
// Session — the stateful booking conversation
// ---------------------------------------------------------------------------

// Session tracks a single ride booking conversation for a user.
type Session struct {
	ID              string
	UserID          string
	Stage           string
	PickupText      string
	DropoffText     string
	DepartureAt     *time.Time
	IsScheduled     bool
	PendingQuestion string
	Summary         string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// MissingFields returns the slot names that have not been filled yet.
func (s *Session) MissingFields() []string {
	var missing []string
	if s.PickupText == "" {
		missing = append(missing, "pickup")
	}
	if s.DropoffText == "" {
		missing = append(missing, "dropoff")
	}
	if s.DepartureAt == nil {
		missing = append(missing, "departure_time")
	}
	return missing
}

// AllFieldsPresent returns true when every required slot is filled.
func (s *Session) AllFieldsPresent() bool {
	return s.PickupText != "" && s.DropoffText != "" && s.DepartureAt != nil
}

// ---------------------------------------------------------------------------
// API request / response
// ---------------------------------------------------------------------------

// MessageRequest is the JSON body sent by the frontend.
type MessageRequest struct {
	Message     string `json:"message"`
	SessionID   string `json:"session_id,omitempty"`
	ContextInfo string `json:"context_info,omitempty"`
}

// MessageResponse is returned to the frontend after processing.
type MessageResponse struct {
	Status  string         `json:"status"` // clarification | confirmation | completed | cancelled | chat
	Reply   string         `json:"reply"`
	Session *SessionView   `json:"session,omitempty"`
	Booking *BookingResult `json:"booking,omitempty"`
}

// SessionView is a read-only snapshot of the session exposed to the frontend.
type SessionView struct {
	ID            string            `json:"id"`
	Stage         string            `json:"stage"`
	KnownFields   map[string]string `json:"known_fields"`
	MissingFields []string          `json:"missing_fields"`
}

// BookingResult is included in the response when a ride order is created.
type BookingResult struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

// NewSessionView builds a SessionView from a Session.
func NewSessionView(s *Session) *SessionView {
	known := make(map[string]string)
	if s.PickupText != "" {
		known["pickup_text"] = s.PickupText
	}
	if s.DropoffText != "" {
		known["dropoff_text"] = s.DropoffText
	}
	if s.DepartureAt != nil {
		known["departure_at"] = s.DepartureAt.Format(time.RFC3339)
	}
	return &SessionView{
		ID:            s.ID,
		Stage:         s.Stage,
		KnownFields:   known,
		MissingFields: s.MissingFields(),
	}
}

// ---------------------------------------------------------------------------
// AI parser contract
// ---------------------------------------------------------------------------

// ParserRequest is the payload sent to the AI planner/parser.
type ParserRequest struct {
	UserMessage  string            `json:"user_message"`
	SessionState map[string]string `json:"session_state"`
	ContextInfo  string            `json:"context_info,omitempty"`
}

// ParserResponse is the structured output expected from the AI parser.
type ParserResponse struct {
	Intent            string  `json:"intent"` // booking | clarification | chat | cancel | completed
	Reply             string  `json:"reply"`
	PickupText        *string `json:"pickup_text,omitempty"`
	DropoffText       *string `json:"dropoff_text,omitempty"`
	DepartureAt       *string `json:"departure_at,omitempty"` // RFC3339
	MissingFields     []string `json:"missing_fields,omitempty"`
	NeedsConfirmation bool    `json:"needs_confirmation"`
	ReadyToBook       bool    `json:"ready_to_book"`
}
