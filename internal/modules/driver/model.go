// README: Driver aggregate model and sentinel errors.
package driver

import (
	"errors"
	"time"

	"ark/internal/types"
)

const (
	StatusAvailable = "available"
	StatusOnTrip    = "on_trip"
	StatusOffline   = "offline"
)

var (
	ErrNotFound   = errors.New("driver not found")
	ErrBadRequest = errors.New("bad request")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("driver already exists")
)

// Driver holds the driver-specific attributes associated with a user account.
type Driver struct {
	ID            types.ID
	LicenseNumber string
	VehicleID     *types.ID
	Rating        float64
	Status        string
	OnboardedAt   time.Time
}
