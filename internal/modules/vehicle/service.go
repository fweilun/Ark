// README: Vehicle service implements CRUD for vehicles.
package vehicle

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound   = errors.New("vehicle not found")
	ErrBadRequest = errors.New("bad request")
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

type CreateCommand struct {
	DriverID         *string
	Make             string
	Model            string
	LicensePlate     string
	Capacity         int
	VehicleType      string // 'sedan', 'suv', 'bike'
	RegistrationDate time.Time
}

type UpdateCommand struct {
	VehicleID        int64
	DriverID         *string
	Make             string
	Model            string
	LicensePlate     string
	Capacity         int
	VehicleType      string
	RegistrationDate time.Time
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (int64, error) {
	if cmd.Make == "" || cmd.Model == "" || cmd.LicensePlate == "" || cmd.VehicleType == "" {
		return 0, ErrBadRequest
	}
	switch cmd.VehicleType {
	case "sedan", "suv", "bike":
	default:
		return 0, ErrBadRequest
	}
	if cmd.Capacity <= 0 {
		cmd.Capacity = 4
	}
	return s.store.Create(ctx, &Vehicle{
		DriverID:         cmd.DriverID,
		Make:             cmd.Make,
		Model:            cmd.Model,
		LicensePlate:     cmd.LicensePlate,
		Capacity:         cmd.Capacity,
		VehicleType:      cmd.VehicleType,
		RegistrationDate: cmd.RegistrationDate,
	})
}

func (s *Service) Get(ctx context.Context, vehicleID int64) (*Vehicle, error) {
	return s.store.Get(ctx, vehicleID)
}

func (s *Service) Update(ctx context.Context, cmd UpdateCommand) error {
	if cmd.VehicleID == 0 {
		return ErrBadRequest
	}
	return s.store.Update(ctx, &Vehicle{
		VehicleID:        cmd.VehicleID,
		DriverID:         cmd.DriverID,
		Make:             cmd.Make,
		Model:            cmd.Model,
		LicensePlate:     cmd.LicensePlate,
		Capacity:         cmd.Capacity,
		VehicleType:      cmd.VehicleType,
		RegistrationDate: cmd.RegistrationDate,
	})
}

func (s *Service) Delete(ctx context.Context, vehicleID int64) error {
	return s.store.Delete(ctx, vehicleID)
}
