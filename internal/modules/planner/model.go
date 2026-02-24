package planner

import "time"

// Status represents the lifecycle state of a Plan.
type Status string

const (
	// StatusDraft is the initial state — plan created but not yet confirmed.
	StatusDraft Status = "draft"
	// StatusConfirmed means the passenger has confirmed the plan.
	StatusConfirmed Status = "confirmed"
	// StatusCancelled means the plan was cancelled before execution.
	StatusCancelled Status = "cancelled"
	// StatusCompleted means the trip corresponding to this plan has finished.
	StatusCompleted Status = "completed"
)

// Plan represents a scheduled trip plan created by the AI secretary on behalf of a passenger.
type Plan struct {
	ID          string    `json:"id"`
	PassengerID string    `json:"passenger_id"`
	Origin      string    `json:"origin"`
	Destination string    `json:"destination"`
	PickupAt    time.Time `json:"pickup_at"`
	Status      Status    `json:"status"`
	// Notes contains any special instructions (e.g. pet, passenger count, intermediate stop).
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
