// README: Typed HTTP response DTOs shared across handlers. These structs
// replace ad-hoc map[string]any payloads so every endpoint has a stable,
// documented JSON shape.
package dto

import (
	"time"

	"ark/internal/types"
)

// OrderResponse is returned by order create/transition endpoints. It carries
// the minimum client-facing fields describing an order mutation result.
//
// Amount-bearing fields (if any are added later) are expressed in TWD cents —
// see internal/types/money.go.
type OrderResponse struct {
	OrderID    string `json:"order_id,omitempty"`
	Status     string `json:"status"`
	LateCancel *bool  `json:"late_cancel,omitempty"`
}

// OrderStatusResponse is returned by GET /api/orders/:id/status.
type OrderStatusResponse struct {
	OrderID       string `json:"order_id"`
	Status        string `json:"status"`
	StatusVersion int    `json:"status_version"`
	DriverID      string `json:"driver_id,omitempty"`
}

// CalendarEventResponse is returned by calendar event endpoints. It
// intentionally carries only the event id.
type CalendarEventResponse struct {
	EventID string `json:"event_id"`
}

// CalendarScheduleResponse is returned by schedule endpoints. The shape is
// intentionally flat (no nested schedule object) so clients can read fields
// directly off the response body.
type CalendarScheduleResponse struct {
	UID       string  `json:"uid"`
	EventID   string  `json:"event_id"`
	TiedOrder *string `json:"tied_order"`
}

// UserResponse is the canonical user payload returned by /api/users and /api/me.
type UserResponse struct {
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	UserType  string `json:"user_type"`
	CreatedAt string `json:"created_at"`
}

// DriverResponse is returned by POST /api/driver/create.
type DriverResponse struct {
	DriverID      string  `json:"driver_id"`
	LicenseNumber string  `json:"license_number"`
	Status        string  `json:"status"`
	Rating        float64 `json:"rating"`
	OnboardedAt   string  `json:"onboarded_at"`
}

// AIChatResponse is returned by POST /api/ai/chat. It contains only the reply.
type AIChatResponse struct {
	Reply string `json:"reply"`
}

// RideAssistantSession is a read-only snapshot of the assistant session
// returned to the frontend.
type RideAssistantSession struct {
	ID            string            `json:"id"`
	Stage         string            `json:"stage"`
	KnownFields   map[string]string `json:"known_fields"`
	MissingFields []string          `json:"missing_fields"`
}

// RideAssistantBooking is included when the assistant has created a booking.
type RideAssistantBooking struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

// RideAssistantResponse is returned by POST /api/assistant/ride/messages.
type RideAssistantResponse struct {
	Status  string                `json:"status"`
	Reply   string                `json:"reply"`
	Session *RideAssistantSession `json:"session,omitempty"`
	Booking *RideAssistantBooking `json:"booking,omitempty"`
}

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// ScheduledOrderSummary is the per-row payload used by the list endpoints
// GET /api/orders/scheduled and GET /api/orders/scheduled/available.
//
// The shape deliberately mirrors every json tag on order.Order so existing
// mobile clients continue to see the same field names and optionality.
// Passing through a dedicated DTO (rather than returning *order.Order
// directly) stops the HTTP surface from leaking Go PascalCase and pins the
// wire shape separately from the domain struct, so internal renames can't
// accidentally break the API.
//
// Monetary fields (EstimatedFee, ActualFee, IncentiveBonus) stay in TWD
// minor units (cents; 1/100 TWD) to match the rest of the API — see
// internal/types/money.go for the unit convention.
type ScheduledOrderSummary struct {
	ID                 string      `json:"id"`
	PassengerID        string      `json:"passenger_id"`
	DriverID           *string     `json:"driver_id,omitempty"`
	Status             string      `json:"status"`
	StatusVersion      int         `json:"status_version"`
	Pickup             types.Point `json:"pickup"`
	Dropoff            types.Point `json:"dropoff"`
	RideType           string      `json:"ride_type"`
	EstimatedFee       types.Money `json:"estimated_fee"`
	ActualFee          *types.Money `json:"actual_fee,omitempty"`
	CreatedAt          time.Time   `json:"created_at"`
	MatchedAt          *time.Time  `json:"matched_at,omitempty"`
	AcceptedAt         *time.Time  `json:"accepted_at,omitempty"`
	StartedAt          *time.Time  `json:"started_at,omitempty"`
	CompletedAt        *time.Time  `json:"completed_at,omitempty"`
	CancelledAt        *time.Time  `json:"cancelled_at,omitempty"`
	CancelReason       *string     `json:"cancel_reason,omitempty"`
	OrderType          string      `json:"order_type,omitempty"`
	ScheduledAt        *time.Time  `json:"scheduled_at,omitempty"`
	ScheduleWindowMins *int        `json:"schedule_window_mins,omitempty"`
	CancelDeadlineAt   *time.Time  `json:"cancel_deadline_at,omitempty"`
	IncentiveBonus     int64       `json:"incentive_bonus"`
	AssignedAt         *time.Time  `json:"assigned_at,omitempty"`
}
