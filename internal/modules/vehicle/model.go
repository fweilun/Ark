// README: Vehicle domain model.
package vehicle

import "ark/internal/types"

// VehicleType enumerates the supported vehicle categories.
type VehicleType string

const (
	VehicleTypeSedan VehicleType = "sedan"
	VehicleTypeSUV   VehicleType = "suv"
	VehicleTypeBike  VehicleType = "bike"
)

// Vehicle represents a driver's registered vehicle.
type Vehicle struct {
	ID               types.ID
	DriverID         types.ID
	Make             string
	Model            string
	LicensePlate     string
	Capacity         int
	Type             VehicleType
	RegistrationDate string // YYYY-MM-DD
}
