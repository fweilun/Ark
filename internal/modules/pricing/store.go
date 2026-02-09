// README: Pricing store backed by PostgreSQL.
package pricing

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) GetRate(ctx context.Context, rideType string) (Rate, error) {
	return Rate{}, errors.New("not implemented")
}
