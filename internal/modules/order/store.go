// README: Order store backed by PostgreSQL (minimal methods for MVP).
package order

import (
    "context"
    "errors"

    "github.com/jackc/pgx/v5/pgxpool"

    "ark/internal/types"
)

type Store struct {
    db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
    return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, o *Order) error {
    return errors.New("not implemented")
}

func (s *Store) Get(ctx context.Context, id types.ID) (*Order, error) {
    return nil, errors.New("not implemented")
}

func (s *Store) UpdateStatus(ctx context.Context, id types.ID, from, to Status, version int) error {
    return errors.New("not implemented")
}

func (s *Store) AppendEvent(ctx context.Context, e *Event) error {
    return errors.New("not implemented")
}
