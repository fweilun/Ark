// README: Location data structures for live updates (driver & passenger MVP).
package location

import (
	"time"

	"ark/internal/types"
)

// Snapshot is used for Postgres persistence (future phase).
type Snapshot struct {
	ID         int64
	UserID     types.ID
	UserType   string
	Position   types.Point
	RecordedAt time.Time
}

// UserType distinguishes driver from passenger location updates.
type UserType string

const (
	UserTypeDriver    UserType = "driver"
	UserTypePassenger UserType = "passenger"
)

// LocationPoint represents a single GPS reading.
type LocationPoint struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	TsMs      int64   `json:"ts_ms"`
	AccuracyM float64 `json:"accuracy_m,omitempty"`
}

// LocationUpdate is the base payload for passenger location updates.
type LocationUpdate struct {
	UserID   string        `json:"user_id"`
	UserType UserType      `json:"user_type"`
	Seq      int64         `json:"seq"`
	Point    LocationPoint `json:"point"`
	IsActive bool          `json:"is_active"`
}

// DriverState represents the operational state of a driver.
type DriverState string

const (
	DriverStateOffline   DriverState = "offline"
	DriverStateAvailable DriverState = "available"
	DriverStateOnTrip    DriverState = "ontrip"
)

// DriverLocationUpdate extends LocationUpdate with driver-specific fields.
type DriverLocationUpdate struct {
	LocationUpdate
	DriverState DriverState `json:"driver_state"`
	OrderID     *string     `json:"order_id,omitempty"`
	SpeedMps    *float64    `json:"speed_mps,omitempty"`
	HeadingDeg  *float64    `json:"heading_deg,omitempty"`
}

// LocationMetadata is stored in Redis alongside the GEO entry.
type LocationMetadata struct {
	Lat         float64     `json:"lat"`
	Lng         float64     `json:"lng"`
	TsMs        int64       `json:"ts_ms"`
	LastSeq     int64       `json:"last_seq"`
	IsActive    bool        `json:"is_active"`
	DriverState DriverState `json:"driver_state,omitempty"`
	OrderID     *string     `json:"order_id,omitempty"`
}

// UpdateResult is the response returned by the service layer.
type UpdateResult struct {
	Accepted  bool `json:"accepted"`
	Throttled bool `json:"throttled"`
}
