// README: Location snapshot for persistence and replay; Redis GEO key constants.
package location

import (
	"time"

	"ark/internal/types"
)

const (
	// DriverLocationsKey is the Redis GEO set holding all driver positions.
	DriverLocationsKey = "driver_locations"
	// PassengerLocationsKey is the Redis GEO set holding all passenger positions.
	PassengerLocationsKey = "passenger_locations"

	// locationTTL is the expiry applied to per-user location hashes.
	// Entries older than this are considered stale.
	locationTTL = 5 * time.Minute
)

// Snapshot is written to Postgres for historical replay.
type Snapshot struct {
	ID         int64
	UserID     types.ID
	UserType   string
	Position   types.Point
	RecordedAt time.Time
}

// LocationRecord is returned by GetLocation queries. It carries the cached
// position together with the time it was last updated.
type LocationRecord struct {
	UserID    types.ID
	Position  types.Point
	UpdatedAt time.Time
}
