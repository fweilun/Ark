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

	// Docs aliases from docs/orderflow.md.
	StatusAwaitingDriver  Status = StatusWaiting
	StatusAcceptedWaiting Status = StatusAssigned
)

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

// AllowedTransitions represents the order state flow (diagram) as code.
var AllowedTransitions = map[Status][]Status{
	StatusScheduled:   {StatusWaiting, StatusCancelled, StatusExpired},
	StatusWaiting:     {StatusWaiting, StatusAssigned, StatusApproaching, StatusCancelled, StatusDenied, StatusExpired},
	StatusAssigned:    {StatusApproaching, StatusCancelled},
	StatusApproaching: {StatusArrived, StatusCancelled},
	StatusArrived:     {StatusDriving, StatusCancelled},
	StatusDriving:     {StatusPayment, StatusCancelled},
	StatusPayment:     {StatusComplete},
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
