// README: Location module data models.
package location

import (
	"time"

	"ark/internal/types"
)

// ---------------------------------------------------------------------------
// Persistent models
// ---------------------------------------------------------------------------

type Snapshot struct {
	ID         int64
	UserID     types.ID
	UserType   string
	Position   types.Point
	RecordedAt time.Time
}

// ---------------------------------------------------------------------------
// Request / command types
// ---------------------------------------------------------------------------

type Update struct {
	UserID   types.ID
	UserType string
	Position types.Point
}

// ---------------------------------------------------------------------------
// Public result types
// ---------------------------------------------------------------------------

// DriverLocation represents a driver's position with computed distance.
type DriverLocation struct {
	DriverID types.ID
	Lat      float64
	Lng      float64
	Distance float64 // km from the queried origin
}

// PassengerLocation represents a passenger's position with computed distance.
type PassengerLocation struct {
	PassengerID types.ID
	Lat         float64
	Lng         float64
	Distance    float64
	Status      string
}

// GeoEntry is a single user position used for batch writes to the Redis GEO set.
type GeoEntry struct {
	ID  types.ID
	Pos types.Point
}

// NearbyUser is the intermediate result from GetNearbyUsersFromRedis.
// The service converts it into DriverLocation or PassengerLocation.
type NearbyUser struct {
	ID       types.ID
	Lat      float64
	Lng      float64
	Distance float64 // km, as returned by Redis GEOSEARCH
}
