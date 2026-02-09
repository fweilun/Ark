// README: Location store backed by Redis GEO and Postgres snapshots.
package location

import (
    "context"
    "errors"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"

    "ark/internal/types"
)

type Store struct {
    db    *pgxpool.Pool
    redis *redis.Client
}

func NewStore(db *pgxpool.Pool, redis *redis.Client) *Store {
    return &Store{db: db, redis: redis}
}

func (s *Store) SetGeo(ctx context.Context, id types.ID, pos types.Point, userType string) error {
    return errors.New("not implemented")
}

func (s *Store) AppendSnapshot(ctx context.Context, snap Snapshot) error {
    return errors.New("not implemented")
}
