// README: Rating store backed by PostgreSQL.
package rating

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

func (s *Store) Create(ctx context.Context, r *Rating) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO ratings (trip_id, rider_rating, driver_rating, comments)
		VALUES ($1, $2, $3, $4)
		RETURNING rating_id`,
		r.TripID,
		nullableInt(r.RiderRating),
		nullableInt(r.DriverRating),
		r.Comments,
	).Scan(&id)
	return id, err
}

func (s *Store) Get(ctx context.Context, ratingID int64) (*Rating, error) {
	row := s.db.QueryRow(ctx, `
		SELECT rating_id, trip_id, rider_rating, driver_rating, comments
		FROM ratings WHERE rating_id = $1`, ratingID,
	)
	return scanRating(row)
}

func (s *Store) GetByTrip(ctx context.Context, tripID string) (*Rating, error) {
	row := s.db.QueryRow(ctx, `
		SELECT rating_id, trip_id, rider_rating, driver_rating, comments
		FROM ratings WHERE trip_id = $1`, tripID,
	)
	return scanRating(row)
}

func scanRating(row interface {
	Scan(...any) error
}) (*Rating, error) {
	var r Rating
	var riderRating, driverRating sql.NullInt32
	err := row.Scan(&r.RatingID, &r.TripID, &riderRating, &driverRating, &r.Comments)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if riderRating.Valid {
		r.RiderRating = int(riderRating.Int32)
	}
	if driverRating.Valid {
		r.DriverRating = int(driverRating.Int32)
	}
	return &r, nil
}

func nullableInt(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}
