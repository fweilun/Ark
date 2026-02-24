// README: Matching store backed by Redis GEO and sets.
package matching

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"ark/internal/types"
)

const (
	geoDriversKey     = "geo:drivers"
	notifiedKeyPrefix = "notified:"
)

type Store struct {
	redis *redis.Client
}

func NewStore(redis *redis.Client) *Store {
	return &Store{redis: redis}
}

// AddCandidate registers a driver in the Redis GEO set.
// Passenger candidates are tracked by waiting orders in PostgreSQL; this is a no-op for them.
func (s *Store) AddCandidate(ctx context.Context, c Candidate) error {
	if c.Type != CandidateDriver {
		return nil
	}
	return s.redis.GeoAdd(ctx, geoDriversKey, &redis.GeoLocation{
		Name:      string(c.ID),
		Longitude: c.Position.Lng,
		Latitude:  c.Position.Lat,
	}).Err()
}

// RemoveCandidate removes a driver from the Redis GEO set.
func (s *Store) RemoveCandidate(ctx context.Context, id types.ID) error {
	return s.redis.ZRem(ctx, geoDriversKey, string(id)).Err()
}

// NearbyDrivers returns IDs of drivers within radiusKm of point p, sorted by distance ascending.
func (s *Store) NearbyDrivers(ctx context.Context, p types.Point, radiusKm float64) ([]types.ID, error) {
	res, err := s.redis.GeoSearch(ctx, geoDriversKey, &redis.GeoSearchQuery{
		Longitude:  p.Lng,
		Latitude:   p.Lat,
		Radius:     radiusKm,
		RadiusUnit: "km",
		Sort:       "ASC",
	}).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]types.ID, len(res))
	for i, name := range res {
		ids[i] = types.ID(name)
	}
	return ids, nil
}

// MarkNotified records that an order has been pushed to drivers, with a TTL after which retrying is allowed.
func (s *Store) MarkNotified(ctx context.Context, orderID types.ID, ttl time.Duration) error {
	return s.redis.SetEx(ctx, notifiedKeyPrefix+string(orderID), 1, ttl).Err()
}

// IsNotified reports whether an order was already pushed to drivers within the current TTL window.
func (s *Store) IsNotified(ctx context.Context, orderID types.ID) (bool, error) {
	n, err := s.redis.Exists(ctx, notifiedKeyPrefix+string(orderID)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
