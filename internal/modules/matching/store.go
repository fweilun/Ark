// README: Matching store backed by Redis GEO and sets.
package matching

import (
    "context"
    "errors"

    "github.com/redis/go-redis/v9"

    "ark/internal/types"
)

type Store struct {
    redis *redis.Client
}

func NewStore(redis *redis.Client) *Store {
    return &Store{redis: redis}
}

func (s *Store) AddCandidate(ctx context.Context, c Candidate) error {
    return errors.New("not implemented")
}

func (s *Store) RemoveCandidate(ctx context.Context, id types.ID) error {
    return errors.New("not implemented")
}

func (s *Store) NearbyDrivers(ctx context.Context, p types.Point, radiusKm float64) ([]types.ID, error) {
    return nil, errors.New("not implemented")
}
