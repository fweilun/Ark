// README: Matching store backed by Redis GEO and sets.
package matching

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"ark/internal/types"
)

const (
	driverGeoKey       = "matching:drivers"
	dispatchKeyPrefix  = "matching:order:%s:dispatched_at"
	broadcastKeyPrefix = "matching:order:%s:broadcast"
	// TTL for dispatch and broadcast keys (orders should resolve well within 7 days).
	keyTTL = 7 * 24 * time.Hour
)

type Store struct {
	redis *redis.Client
}

func NewStore(redis *redis.Client) *Store {
	return &Store{redis: redis}
}

func (s *Store) AddCandidate(ctx context.Context, c Candidate) error {
	return s.redis.GeoAdd(ctx, driverGeoKey, &redis.GeoLocation{
		Name:      string(c.ID),
		Longitude: c.Position.Lng,
		Latitude:  c.Position.Lat,
	}).Err()
}

func (s *Store) RemoveCandidate(ctx context.Context, id types.ID) error {
	return s.redis.ZRem(ctx, driverGeoKey, string(id)).Err()
}

func (s *Store) NearbyDrivers(ctx context.Context, p types.Point, radiusKm float64) ([]types.ID, error) {
	results, err := s.redis.GeoSearch(ctx, driverGeoKey, &redis.GeoSearchQuery{
		Longitude:  p.Lng,
		Latitude:   p.Lat,
		Radius:     radiusKm,
		RadiusUnit: "km",
		Sort:       "ASC",
	}).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]types.ID, len(results))
	for i, r := range results {
		ids[i] = types.ID(r)
	}
	return ids, nil
}

// RecordDispatch records the dispatch timestamp and the set of notified drivers for an order.
func (s *Store) RecordDispatch(ctx context.Context, orderID types.ID, driverIDs []types.ID) error {
	pipe := s.redis.Pipeline()
	pipe.Set(ctx, dispatchedAtKey(orderID), time.Now().UTC().Format(time.RFC3339), keyTTL)
	if len(driverIDs) > 0 {
		members := make([]interface{}, len(driverIDs))
		for i, d := range driverIDs {
			members[i] = string(d)
		}
		notifiedKey := fmt.Sprintf("matching:order:%s:notified", string(orderID))
		pipe.SAdd(ctx, notifiedKey, members...)
		pipe.Expire(ctx, notifiedKey, keyTTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// GetDispatchedAt returns when the order was first dispatched, and whether it has been dispatched.
func (s *Store) GetDispatchedAt(ctx context.Context, orderID types.ID) (time.Time, bool, error) {
	val, err := s.redis.Get(ctx, dispatchedAtKey(orderID)).Result()
	if err == redis.Nil {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}, false, err
	}
	return t, true, nil
}

// MarkOrderBroadcast marks an order as having been broadcast to all drivers.
func (s *Store) MarkOrderBroadcast(ctx context.Context, orderID types.ID) error {
	return s.redis.Set(ctx, broadcastKey(orderID), "1", keyTTL).Err()
}

// IsOrderBroadcast reports whether an order has been broadcast to all drivers.
func (s *Store) IsOrderBroadcast(ctx context.Context, orderID types.ID) (bool, error) {
	val, err := s.redis.Get(ctx, broadcastKey(orderID)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == "1", nil
}

func dispatchedAtKey(orderID types.ID) string {
	return fmt.Sprintf(dispatchKeyPrefix, string(orderID))
}

func broadcastKey(orderID types.ID) string {
	return fmt.Sprintf(broadcastKeyPrefix, string(orderID))
}
