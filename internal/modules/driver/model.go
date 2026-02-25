// README: Driver model and status definitions.
package driver

import "time"

type Status string

const (
	StatusAvailable Status = "available"
	StatusOnTrip    Status = "on_trip"
	StatusOffline   Status = "offline"
)

// Driver holds driver-specific attributes linked to a User (driver_id = user_id).
type Driver struct {
	DriverID      string
	LicenseNumber string
	VehicleID     *int64 // nullable FK to vehicles.vehicle_id
	Rating        float64
	Status        Status
	OnboardedAt   time.Time
}
