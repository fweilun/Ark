// README: Driver store backed by PostgreSQL.
package driver

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, d *Driver) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO drivers (driver_id, license_number, vehicle_id, rating, status)
		VALUES ($1, $2, $3, $4, $5)`,
		d.DriverID, d.LicenseNumber, d.VehicleID, d.Rating, string(d.Status),
	)
	return err
}

func (s *Store) Get(ctx context.Context, driverID string) (*Driver, error) {
	row := s.db.QueryRow(ctx, `
		SELECT driver_id, license_number, vehicle_id, rating, status, onboarded_at
		FROM drivers WHERE driver_id = $1`, driverID,
	)
	var d Driver
	var vehicleID sql.NullInt64
	err := row.Scan(&d.DriverID, &d.LicenseNumber, &vehicleID, &d.Rating, &d.Status, &d.OnboardedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if vehicleID.Valid {
		v := vehicleID.Int64
		d.VehicleID = &v
	}
	return &d, nil
}

func (s *Store) Update(ctx context.Context, d *Driver) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE drivers SET license_number = $1, vehicle_id = $2
		WHERE driver_id = $3`,
		d.LicenseNumber, d.VehicleID, d.DriverID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, driverID string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM drivers WHERE driver_id = $1`, driverID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateRating(ctx context.Context, driverID string, rating float64) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE drivers SET rating = $1 WHERE driver_id = $2`,
		rating, driverID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus locks the driver row and updates status atomically.
func (s *Store) UpdateStatus(ctx context.Context, driverID string, status Status) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var current Status
	err = tx.QueryRow(ctx, `
		SELECT status FROM drivers WHERE driver_id = $1 FOR UPDATE`, driverID,
	).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if _, err = tx.Exec(ctx, `UPDATE drivers SET status = $1 WHERE driver_id = $2`, string(status), driverID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
