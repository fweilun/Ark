// README: Typed HTTP response DTOs shared across handlers. These structs
// replace ad-hoc map[string]any payloads so every endpoint has a stable,
// documented JSON shape.
package dto

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
