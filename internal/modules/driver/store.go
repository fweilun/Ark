// README: Driver store — PostgreSQL-backed persistence for driver profiles.
package driver

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"ark/internal/types"
)

// DriverStore defines the persistence operations required by the driver Service.
type DriverStore interface {
	Create(ctx context.Context, d *Driver) error
	Get(ctx context.Context, id types.ID) (*Driver, error)
	UpdateRating(ctx context.Context, id types.ID, newRating float64) error
	UpdateStatusWithLock(ctx context.Context, id types.ID, newStatus string) error
}

// Store is the PostgreSQL implementation of DriverStore.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, d *Driver) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO drivers (driver_id, license_number, vehicle_id, rating, status, onboarded_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		string(d.ID), d.LicenseNumber, toStringPtr(d.VehicleID), d.Rating, d.Status, d.OnboardedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return err
	}
	return nil
}

func (s *Store) Get(ctx context.Context, id types.ID) (*Driver, error) {
	row := s.db.QueryRow(ctx, `
		SELECT driver_id, license_number, vehicle_id, rating, status, onboarded_at
		FROM drivers WHERE driver_id = $1`, string(id))

	var d Driver
	var vehicleID sql.NullString
	err := row.Scan(&d.ID, &d.LicenseNumber, &vehicleID, &d.Rating, &d.Status, &d.OnboardedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if vehicleID.Valid {
		v := types.ID(vehicleID.String)
		d.VehicleID = &v
	}
	return &d, nil
}

func (s *Store) UpdateRating(ctx context.Context, id types.ID, newRating float64) error {
	tag, err := s.db.Exec(ctx, `UPDATE drivers SET rating = $1 WHERE driver_id = $2`, newRating, string(id))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatusWithLock updates the driver's status within a transaction using a row-level lock
// (SELECT ... FOR UPDATE) to prevent concurrent conflicting writes.
func (s *Store) UpdateStatusWithLock(ctx context.Context, id types.ID, newStatus string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM drivers WHERE driver_id = $1 FOR UPDATE)`,
		string(id),
	).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}

	if _, err := tx.Exec(ctx,
		`UPDATE drivers SET status = $1 WHERE driver_id = $2`,
		newStatus, string(id),
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func toStringPtr(id *types.ID) *string {
	if id == nil {
		return nil
	}
	s := string(*id)
	return &s
}
