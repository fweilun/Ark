// README: Calendar domain models — Event, Schedule, and OrderEvent.
package calendar

import (
	"time"

	"ark/internal/types"
)

// Event represents a calendar event with a time range, title, and description.
type Event struct {
	ID          types.ID
	From        time.Time
	To          time.Time
	Title       string
	Description string
}

// Schedule links a user to a calendar event (tracks attendance).
type Schedule struct {
	UID     types.ID // user ID
	EventID types.ID // linked event
}

// OrderEvent links a ride order to a calendar event.
// Multiple orders can be linked to a single event (e.g. pickup and dropoff rides).
type OrderEvent struct {
	ID        types.ID
	EventID   types.ID
	OrderID   types.ID
	UID       types.ID
	CreatedAt time.Time
}
