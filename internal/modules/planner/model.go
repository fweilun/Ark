package planner

import "time"

// Status represents the lifecycle state of a Plan.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusConfirmed Status = "confirmed"
	StatusCancelled Status = "cancelled"
	StatusCompleted Status = "completed"
)

// Plan represents a scheduled trip plan created by the AI secretary.
type Plan struct {
	ID          string    `json:"id"`
	PassengerID string    `json:"passenger_id"`
	Origin      string    `json:"origin"`
	Destination string    `json:"destination"`
	PickupAt    time.Time `json:"pickup_at"`
	Status      Status    `json:"status"`
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
