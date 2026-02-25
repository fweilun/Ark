// README: Location snapshot for persistence and replay.
package location

import (
    "time"

    "ark/internal/types"
)

type Snapshot struct {
	ID         int64
	UserID     types.ID
	UserType   string
	Position   types.Point
	RecordedAt time.Time
}

// NearbyUser is the intermediate result from GetNearbyUsersFromRedis.
// The service converts it into DriverLocation or PassengerLocation.
type NearbyUser struct {
	ID       types.ID
	Lat      float64
	Lng      float64
	Distance float64 // km, as returned by Redis GEOSEARCH
}
