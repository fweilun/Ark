// README: Matching candidates for passengers and drivers.
package matching

import (
    "time"

    "ark/internal/types"
)

type CandidateType string

const (
    CandidatePassenger CandidateType = "passenger"
    CandidateDriver    CandidateType = "driver"
)

type Candidate struct {
    ID        types.ID
    Type      CandidateType
    RideTypes []string
    Position  types.Point
    JoinTime  time.Time
}

type MatchResult struct {
    PassengerID types.ID
    DriverID    types.ID
    WaitTimeSec int
}

// OrderNotification tracks how many times a scheduled/waiting order has been
// broadcast to drivers and when the next broadcast window opens (cooldown).
type OrderNotification struct {
    OrderID          types.ID
    NotifyCount      int
    LastNotifiedAt   time.Time
    NextNotifiableAt time.Time
}
