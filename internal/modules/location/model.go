// README: Location snapshot for persistence and replay.
package location

import (
	"time"

	"ark/internal/types"
)

// ---------------------------------------------------------------------------
// Persistent Models
// ---------------------------------------------------------------------------

type Snapshot struct {
	ID         int64
	UserID     types.ID
	UserType   string
	Position   types.Point
	RecordedAt time.Time
}

type Update struct {
	UserID   types.ID
	UserType string
	Position types.Point
}

// ---------------------------------------------------------------------------
// RTDB Data Models
// ---------------------------------------------------------------------------

// rtdbDriverEntry mirrors a single driver entry stored in Firebase RTDB
// under the /driver_locations node.
type rtdbDriverEntry struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Status    string  `json:"status"`
	Timestamp int64   `json:"timestamp"`
}

// rtdbPassengerEntry mirrors a single passenger entry stored in Firebase RTDB
// under the /passenger_locations node.
type rtdbPassengerEntry struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Status    string  `json:"status"`
	Timestamp int64   `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Public Result Types
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

// OrderInfo contains the payload data to send via FCM.
type OrderInfo struct {
	OrderID      types.ID
	PickupLat    float64
	PickupLng    float64
	DropoffLat   float64
	DropoffLng   float64
	EstimatedFee float64
}
