// README: Order aggregate and status definitions.
package order

import (
    "time"

    "ark/internal/types"
)

type Status string

const (
    StatusNone       Status = "none"
    StatusRequested  Status = "request_ride"
    StatusDriverFound Status = "driver_found"
    StatusRideAccepted Status = "ride_accepted"
    StatusTripStarted  Status = "trip_started"
    StatusTripComplete Status = "trip_complete"
    StatusPayment      Status = "payment"
    StatusCancelled  Status = "cancelled"
    StatusRideDenied Status = "ride_denied"
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
    StatusRequested:   {StatusDriverFound, StatusCancelled},
    StatusDriverFound: {StatusRideAccepted, StatusRideDenied},
    StatusRideAccepted:{StatusTripStarted},
    StatusTripStarted: {StatusTripComplete},
    StatusTripComplete:{StatusPayment},
    StatusCancelled:   {StatusRideDenied},
}

func CanTransition(from, to Status) bool {
    next, ok := AllowedTransitions[from]
    if !ok {
        return false
    }
    for _, s := range next {
        if s == to {
            return true
        }
    }
    return false
}
