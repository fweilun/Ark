// README: Order aggregate and status definitions.
package order

import (
	"time"

	"ark/internal/types"
)

type Status string

const (
	StatusNone        Status = "none"
	StatusScheduled   Status = "scheduled" // scheduled order created
	StatusWaiting     Status = "waiting"   // awaiting driver match
	StatusAssigned    Status = "assigned"  // accepted waiting (driver accepted, not departed)
	StatusApproaching Status = "approaching"
	StatusArrived     Status = "arrived"
	StatusDriving     Status = "driving"
	StatusPayment     Status = "payment"
	StatusComplete    Status = "complete"
	StatusCancelled   Status = "cancelled"
	StatusDenied      Status = "denied"
	StatusExpired     Status = "expired"
)

// type OrderType string

// const (
// 	Scheduled OrderType = "scheduled"
// 	Instant   OrderType = "instant"
// )

type Order struct {
	ID            types.ID    `json:"id"`
	PassengerID   types.ID    `json:"passenger_id"`
	DriverID      *types.ID   `json:"driver_id,omitempty"`
	Status        Status      `json:"status"`
	StatusVersion int         `json:"status_version"`
	Pickup        types.Point `json:"pickup"`
	Dropoff       types.Point `json:"dropoff"`
	RideType      string      `json:"ride_type"`
	// EstimatedFee is the pre-trip fare quote in TWD minor units
	// (cents; 1/100 TWD). See internal/types/money.go for the unit convention.
	EstimatedFee types.Money `json:"estimated_fee"`
	// ActualFee is the post-trip charged fare in TWD minor units. Nil until
	// the trip completes. See internal/types/money.go for the unit convention.
	ActualFee    *types.Money `json:"actual_fee,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	MatchedAt    *time.Time   `json:"matched_at,omitempty"`
	AcceptedAt   *time.Time   `json:"accepted_at,omitempty"`
	StartedAt    *time.Time   `json:"started_at,omitempty"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
	CancelledAt  *time.Time   `json:"cancelled_at,omitempty"`
	CancelReason *string      `json:"cancel_reason,omitempty"`
	// Scheduled-order fields (zero/nil for instant orders).
	OrderType          string     `json:"order_type,omitempty"`
	ScheduledAt        *time.Time `json:"scheduled_at,omitempty"`
	ScheduleWindowMins *int       `json:"schedule_window_mins,omitempty"`
	CancelDeadlineAt   *time.Time `json:"cancel_deadline_at,omitempty"`
	// IncentiveBonus is an accumulated driver-incentive kicker added when
	// scheduled orders approach their pickup window. Value is in TWD minor
	// units (cents; 1/100 TWD). See internal/types/money.go.
	IncentiveBonus int64      `json:"incentive_bonus"`
	AssignedAt     *time.Time `json:"assigned_at,omitempty"`
	history        []Event    `json:"-"`
}

type Event struct {
	ID         int64
	OrderID    types.ID
	FromStatus Status
	ToStatus   Status
	ActorType  string
	ActorID    *types.ID
	CreatedAt  time.Time
}

// AllowedTransitions represents the order state flow as code (see docs/orderflow.md mermaid diagram).
// Actor-specific semantics (passenger/driver/system) are enforced by the service layer;
// this map defines which state pairs are structurally valid.
// TODO: (specific status) ex. Payment fail,
var AllowedTransitions = map[Status][]Status{
	// Scheduled order is assigned, cancelled. We obmit the waiting, expired for now since the schedule order is important.
	StatusScheduled: {StatusAssigned, StatusCancelled},
	// Awaiting a driver: realtime → Approaching on accept, scheduled → Assigned on accept,
	// self-loop on matching retry, → Denied on driver decline, → Cancelled on passenger cancel,
	// → Expired on matching timeout.
	StatusWaiting: {StatusWaiting, StatusApproaching, StatusCancelled, StatusExpired},
	// Driver accepted a scheduled order but has not departed; can start trip (→ Approaching), cancel,
	// or be re-opened by driver cancel (→ Scheduled).
	StatusAssigned: {StatusApproaching, StatusCancelled, StatusScheduled},
	// Driver en route: arrives at pickup (→ Arrived), passenger or driver cancels (→ Cancelled),
	// or driver cancels for re-matching (→ Waiting).
	StatusApproaching: {StatusArrived, StatusCancelled, StatusWaiting},
	// Driver at pickup: passenger boards (→ Driving) or either party cancels.
	StatusArrived: {StatusDriving, StatusCancelled},
	// Trip in progress: drop-off (→ Payment) or driver cancels.
	StatusDriving: {StatusPayment, StatusCancelled},
	// Awaiting payment confirmation.
	StatusPayment: {StatusComplete},
}

var allowedTransitionSet = buildTransitionSet(AllowedTransitions)

func buildTransitionSet(transitions map[Status][]Status) map[Status]map[Status]struct{} {
	set := make(map[Status]map[Status]struct{}, len(transitions))
	for from, tos := range transitions {
		next := make(map[Status]struct{}, len(tos))
		for _, to := range tos {
			next[to] = struct{}{}
		}
		set[from] = next
	}
	return set
}

// CanTransition checks if a transition of order is valid.
func CanTransition(from, to Status) bool {
	next, ok := allowedTransitionSet[from]
	if !ok {
		return false
	}
	_, ok = next[to]
	return ok
}
