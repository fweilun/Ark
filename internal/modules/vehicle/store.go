// README: Vehicle store backed by PostgreSQL.
package vehicle

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ark/internal/types"
)

// Store handles persistence for vehicles.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a Store backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Create inserts a new vehicle record.
func (s *Store) Create(ctx context.Context, v *Vehicle) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO vehicles (id, driver_id, make, model, license_plate, capacity, vehicle_type, registration_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		string(v.ID), string(v.DriverID), v.Make, v.Model, v.LicensePlate,
		v.Capacity, string(v.Type), v.RegistrationDate,
	)
	return err
}

// GetByDriverID retrieves the vehicle bound to the given driver.
func (s *Store) GetByDriverID(ctx context.Context, driverID types.ID) (*Vehicle, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, driver_id, make, model, license_plate, capacity, vehicle_type, registration_date
		FROM vehicles
		WHERE driver_id = $1`,
		string(driverID),
	)
	var v Vehicle
	err := row.Scan(&v.ID, &v.DriverID, &v.Make, &v.Model, &v.LicensePlate,
		&v.Capacity, &v.Type, &v.RegistrationDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// Update modifies the mutable fields of a driver's vehicle (excludes registration_date).
func (s *Store) Update(ctx context.Context, v *Vehicle) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE vehicles
		SET make = $1, model = $2, license_plate = $3, capacity = $4, vehicle_type = $5
		WHERE driver_id = $6`,
		v.Make, v.Model, v.LicensePlate, v.Capacity, string(v.Type), string(v.DriverID),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByDriverID removes the vehicle record associated with the given driver.
func (s *Store) DeleteByDriverID(ctx context.Context, driverID types.ID) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM vehicles WHERE driver_id = $1`, string(driverID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
