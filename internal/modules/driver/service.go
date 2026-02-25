// README: Driver service implements CRUD, rating updates, and status updates.
package driver

import (
	"context"
	"errors"
)

var (
	ErrNotFound      = errors.New("driver not found")
	ErrBadRequest    = errors.New("bad request")
	ErrInvalidStatus = errors.New("invalid status")
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

type CreateCommand struct {
	// DriverID is the Firebase UID, parsed from the Authorization header by the auth middleware.
	DriverID      string
	LicenseNumber string
}

type UpdateCommand struct {
	DriverID      string
	LicenseNumber string
	VehicleID     *int64
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) error {
	if cmd.DriverID == "" || cmd.LicenseNumber == "" {
		return ErrBadRequest
	}
	return s.store.Create(ctx, &Driver{
		DriverID:      cmd.DriverID,
		LicenseNumber: cmd.LicenseNumber,
		Rating:        0.0,
		Status:        StatusOffline,
	})
}

func (s *Service) Get(ctx context.Context, driverID string) (*Driver, error) {
	if driverID == "" {
		return nil, ErrBadRequest
	}
	return s.store.Get(ctx, driverID)
}

func (s *Service) Update(ctx context.Context, cmd UpdateCommand) error {
	if cmd.DriverID == "" {
		return ErrBadRequest
	}
	return s.store.Update(ctx, &Driver{
		DriverID:      cmd.DriverID,
		LicenseNumber: cmd.LicenseNumber,
		VehicleID:     cmd.VehicleID,
	})
}

func (s *Service) Delete(ctx context.Context, driverID string) error {
	if driverID == "" {
		return ErrBadRequest
	}
	return s.store.Delete(ctx, driverID)
}

// UpdateRating updates the average rating for a driver.
func (s *Service) UpdateRating(ctx context.Context, driverID string, rating float64) error {
	if driverID == "" {
		return ErrBadRequest
	}
	if rating < 0 || rating > 5 {
		return ErrBadRequest
	}
	return s.store.UpdateRating(ctx, driverID, rating)
}

// UpdateStatus updates the driver's availability status with a row-level lock.
func (s *Service) UpdateStatus(ctx context.Context, driverID string, status Status) error {
	if driverID == "" {
		return ErrBadRequest
	}
	switch status {
	case StatusAvailable, StatusOnTrip, StatusOffline:
	default:
		return ErrInvalidStatus
	}
	return s.store.UpdateStatus(ctx, driverID, status)
}
