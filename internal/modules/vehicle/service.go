// README: Vehicle service — registers, retrieves, updates, and removes driver vehicles.
package vehicle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"ark/internal/types"
)

var (
	ErrNotFound   = errors.New("vehicle: not found")
	ErrBadRequest = errors.New("vehicle: bad request")
	ErrConflict   = errors.New("vehicle: driver already has a vehicle")
)

// Service orchestrates vehicle business logic.
type Service struct {
	store *Store
}

// NewService creates a Service backed by the given Store.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// CreateVehicleCommand holds the fields supplied by the driver when registering a vehicle.
// driver_id is NOT part of the command; it is injected from auth context by the handler.
type CreateVehicleCommand struct {
	Make             string
	Model            string
	LicensePlate     string
	Capacity         int
	VehicleType      string
	RegistrationDate string // YYYY-MM-DD
}

// UpdateVehicleCommand holds the mutable fields a driver may change.
// registration_date is excluded per spec.
type UpdateVehicleCommand struct {
	Make         string
	Model        string
	LicensePlate string
	Capacity     int
	VehicleType  string
}

// CreateVehicle registers a new vehicle for the authenticated driver.
func (s *Service) CreateVehicle(ctx context.Context, driverID types.ID, cmd CreateVehicleCommand) (*Vehicle, error) {
	if driverID == "" || cmd.Make == "" || cmd.Model == "" || cmd.LicensePlate == "" || cmd.Capacity <= 0 {
		return nil, ErrBadRequest
	}
	if !isValidVehicleType(cmd.VehicleType) {
		return nil, ErrBadRequest
	}
	v := &Vehicle{
		ID:               newID(),
		DriverID:         driverID,
		Make:             cmd.Make,
		Model:            cmd.Model,
		LicensePlate:     cmd.LicensePlate,
		Capacity:         cmd.Capacity,
		Type:             VehicleType(cmd.VehicleType),
		RegistrationDate: cmd.RegistrationDate,
	}
	if err := s.store.Create(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// GetDriverVehicle returns the vehicle currently bound to the given driver.
// Also used by order and matching modules.
func (s *Service) GetDriverVehicle(ctx context.Context, driverID types.ID) (*Vehicle, error) {
	if driverID == "" {
		return nil, ErrBadRequest
	}
	return s.store.GetByDriverID(ctx, driverID)
}

// UpdateVehicleInfo updates the mutable fields of the driver's vehicle.
func (s *Service) UpdateVehicleInfo(ctx context.Context, driverID types.ID, cmd UpdateVehicleCommand) (*Vehicle, error) {
	if driverID == "" || cmd.Make == "" || cmd.Model == "" || cmd.LicensePlate == "" || cmd.Capacity <= 0 {
		return nil, ErrBadRequest
	}
	if !isValidVehicleType(cmd.VehicleType) {
		return nil, ErrBadRequest
	}
	v := &Vehicle{
		DriverID:     driverID,
		Make:         cmd.Make,
		Model:        cmd.Model,
		LicensePlate: cmd.LicensePlate,
		Capacity:     cmd.Capacity,
		Type:         VehicleType(cmd.VehicleType),
	}
	if err := s.store.Update(ctx, v); err != nil {
		return nil, err
	}
	return s.store.GetByDriverID(ctx, driverID)
}

// DeleteVehicle removes the vehicle record for the given driver.
func (s *Service) DeleteVehicle(ctx context.Context, driverID types.ID) error {
	if driverID == "" {
		return ErrBadRequest
	}
	return s.store.DeleteByDriverID(ctx, driverID)
}

func isValidVehicleType(t string) bool {
	switch VehicleType(t) {
	case VehicleTypeSedan, VehicleTypeSUV, VehicleTypeBike:
		return true
	}
	return false
}

func newID() types.ID {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return types.ID(hex.EncodeToString(b[:]))
}
