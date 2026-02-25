// README: Vehicle store backed by PostgreSQL.
package vehicle

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

func (s *Store) Create(ctx context.Context, v *Vehicle) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO vehicles (driver_id, make, model, license_plate, capacity, vehicle_type, registration_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING vehicle_id`,
		v.DriverID, v.Make, v.Model, v.LicensePlate, v.Capacity, v.VehicleType, v.RegistrationDate,
	).Scan(&id)
	return id, err
}

func (s *Store) Get(ctx context.Context, vehicleID int64) (*Vehicle, error) {
	row := s.db.QueryRow(ctx, `
		SELECT vehicle_id, driver_id, make, model, license_plate, capacity, vehicle_type, registration_date
		FROM vehicles WHERE vehicle_id = $1`, vehicleID,
	)
	var v Vehicle
	var driverID sql.NullString
	err := row.Scan(&v.VehicleID, &driverID, &v.Make, &v.Model, &v.LicensePlate, &v.Capacity, &v.VehicleType, &v.RegistrationDate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if driverID.Valid {
		v.DriverID = &driverID.String
	}
	return &v, nil
}

func (s *Store) Update(ctx context.Context, v *Vehicle) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE vehicles
		SET driver_id = $1, make = $2, model = $3, license_plate = $4,
		    capacity = $5, vehicle_type = $6, registration_date = $7
		WHERE vehicle_id = $8`,
		v.DriverID, v.Make, v.Model, v.LicensePlate,
		v.Capacity, v.VehicleType, v.RegistrationDate, v.VehicleID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, vehicleID int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM vehicles WHERE vehicle_id = $1`, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
