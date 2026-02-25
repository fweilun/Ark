// README: Location store backed by Redis GEO and Postgres snapshots.
package location

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"ark/internal/types"
)

const (
	geoKeyDrivers    = "geo:drivers"
	geoKeyPassengers = "geo:passengers"
	statusTTL        = 60 * time.Second
)

type Store struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

func NewStore(db *pgxpool.Pool, redis *redis.Client) *Store {
	return &Store{db: db, redis: redis}
}

// geoSetKey returns the Redis sorted-set key for the GEO index of a user type.
func geoSetKey(userType string) string {
	if userType == "passenger" {
		return geoKeyPassengers
	}
	return geoKeyDrivers
}

// statusKey returns the per-user TTL key used to detect active users.
func statusKey(userType string, id types.ID) string {
	return userType + "_status:" + string(id)
}

// SetGeo writes the user's position to the Redis GEO sorted set and refreshes
// their status key with a 60-second TTL. Both operations run in a single
// pipeline to minimise round-trips.
func (s *Store) SetGeo(ctx context.Context, id types.ID, pos types.Point, userType string) error {
	pipe := s.redis.Pipeline()
	pipe.GeoAdd(ctx, geoSetKey(userType), &redis.GeoLocation{
		Name:      string(id),
		Longitude: pos.Lng,
		Latitude:  pos.Lat,
	})
	pipe.Set(ctx, statusKey(userType, id), "1", statusTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("SetGeo %s %s: %w", userType, id, err)
	}
	return nil
}

// GetNearbyUsersFromRedis performs a GEOSEARCH for users within radiusKm of
// (lat, lng) and filters out any whose status key has expired (i.e. offline).
// Results are returned sorted by distance ascending (closest first).
func (s *Store) GetNearbyUsersFromRedis(ctx context.Context, lat, lng, radiusKm float64, userType string) ([]NearbyUser, error) {
	results, err := s.redis.GeoSearchLocation(ctx, geoSetKey(userType), &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  lng,
			Latitude:   lat,
			Radius:     radiusKm,
			RadiusUnit: "km",
			Sort:       "ASC",
		},
		WithCoord: true,
		WithDist:  true,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("GEOSEARCH %s: %w", userType, err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	// Batch-check status keys; nil means the TTL expired → user is offline.
	statusKeys := make([]string, len(results))
	for i, r := range results {
		statusKeys[i] = statusKey(userType, types.ID(r.Name))
	}
	statuses, err := s.redis.MGet(ctx, statusKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("MGET status %s: %w", userType, err)
	}

	var expired []interface{}
	var active []NearbyUser
	for i, status := range statuses {
		if status == nil {
			expired = append(expired, results[i].Name)
			continue
		}
		r := results[i]
		active = append(active, NearbyUser{
			ID:       types.ID(r.Name),
			Lat:      r.Latitude,
			Lng:      r.Longitude,
			Distance: r.Dist,
		})
	}

	// Lazy deletion: remove stale GEO members whose status TTL has expired.
	// A detached context is used so the cleanup is not cancelled if the request
	// context ends before the goroutine executes.
	if len(expired) > 0 {
		key := geoSetKey(userType)
		go func() {
			if err := s.redis.ZRem(context.Background(), key, expired...).Err(); err != nil {
				log.Printf("location: lazy ZRem failed for %s: %v", key, err)
			}
		}()
	}

	return active, nil
}

func (s *Store) AppendSnapshot(ctx context.Context, snap Snapshot) error {
	return errors.New("not implemented")
}
