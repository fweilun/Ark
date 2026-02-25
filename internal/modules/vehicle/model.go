// README: Vehicle model and type definitions.
package vehicle

import "time"

// Vehicle represents a physical vehicle asset managed by the system.
type Vehicle struct {
	VehicleID        int64
	DriverID         *string // nullable FK to users.user_id
	Make             string
	Model            string
	LicensePlate     string
	Capacity         int
	VehicleType      string // 'sedan', 'suv', 'bike'
	RegistrationDate time.Time
}
