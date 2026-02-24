// README: Calendar domain models — Event and Schedule.
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

// Schedule links a user to a calendar event and optionally to a ride order.
type Schedule struct {
	UID       types.ID  // user ID
	EventID   types.ID  // linked event
	TiedOrder *types.ID // optional order reference
}
