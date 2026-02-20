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
	ID            types.ID
	PassengerID   types.ID
	DriverID      *types.ID
	Status        Status
	StatusVersion int
	Pickup        types.Point
	Dropoff       types.Point
	RideType      string
	EstimatedFee  types.Money
	ActualFee     *types.Money
	CreatedAt     time.Time
	MatchedAt     *time.Time
	AcceptedAt    *time.Time
	StartedAt     *time.Time
	CompletedAt   *time.Time
	CancelledAt   *time.Time
	CancelReason  *string
	// Scheduled-order fields (zero/nil for instant orders).
	OrderType          string
	ScheduledAt        *time.Time
	ScheduleWindowMins *int
	CancelDeadlineAt   *time.Time
	IncentiveBonus     int64
	AssignedAt         *time.Time
	history            []Event
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
