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

const (
// notifyInitialCount is the number of drivers to notify on the first dispatch.
notifyInitialCount = 5
// selectPoolSize is how many nearby drivers to sample before picking notifyInitialCount.
selectPoolSize = 10
// broadcastDelay is how long to wait after initial dispatch before opening the order
// to all drivers via the public scheduled list.
broadcastDelay = 30 * time.Second
// oneDayBefore is the lead-time threshold for the extra broadcast.
oneDayBefore = 24 * time.Hour
// broadcastExtraCount is how many additional drivers to notify at the T-24h broadcast.
broadcastExtraCount = 10
// scheduledOrderLookaheadWindow is how far ahead the scheduler looks for orders.
scheduledOrderLookaheadWindow = 7 * 24 * time.Hour
)
