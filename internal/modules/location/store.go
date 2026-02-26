// README: Location store backed by Redis GEO and Postgres snapshots.
package location

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"ark/internal/types"
)

// Store is backed by Redis for live geospatial queries and Postgres for
// historical location snapshots.
type Store struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

// NewStore creates a Store wired to the given Postgres pool and Redis client.
func NewStore(db *pgxpool.Pool, redis *redis.Client) *Store {
	return &Store{db: db, redis: redis}
}

// geoKey returns the Redis GEO set key for the given user type.
func geoKey(userType string) string {
	if userType == "driver" {
		return DriverLocationsKey
	}
	return PassengerLocationsKey
}

// hashKey returns the per-user Redis hash key used to store location metadata
// (lat, lng, updated_at) with a TTL.
func hashKey(userType string, id types.ID) string {
	return fmt.Sprintf("%s_location:%s", userType, string(id))
}

// SetGeo stores the user's current position in two places:
//  1. The shared GEO set (driver_locations / passenger_locations) for GEOSEARCH.
//  2. A per-user hash with a TTL so callers can detect stale entries and
//     retrieve the exact float coordinates via GetLocation.
//
// Note: members in the GEO set are not automatically removed when the per-user
// hash expires. For production use, run a periodic cleanup job that calls
// ZREM on the GEO set for members whose hash no longer exists.
func (s *Store) SetGeo(ctx context.Context, id types.ID, pos types.Point, userType string) error {
	geo := geoKey(userType)
	if err := s.redis.GeoAdd(ctx, geo, &redis.GeoLocation{
		Name:      string(id),
		Longitude: pos.Lng,
		Latitude:  pos.Lat,
	}).Err(); err != nil {
		return fmt.Errorf("geoadd %s: %w", geo, err)
	}

	hk := hashKey(userType, id)
	pipe := s.redis.Pipeline()
	pipe.HSet(ctx, hk,
		"lat", strconv.FormatFloat(pos.Lat, 'f', 7, 64),
		"lng", strconv.FormatFloat(pos.Lng, 'f', 7, 64),
		"updated_at", strconv.FormatInt(time.Now().Unix(), 10),
	)
	pipe.Expire(ctx, hk, locationTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("hset %s: %w", hk, err)
	}
	return nil
}

// GetLocation retrieves the most-recently cached location for a user.
// Returns an error if the entry does not exist or has expired (TTL elapsed).
func (s *Store) GetLocation(ctx context.Context, id types.ID, userType string) (LocationRecord, error) {
	hk := hashKey(userType, id)
	vals, err := s.redis.HGetAll(ctx, hk).Result()
	if err != nil {
		return LocationRecord{}, fmt.Errorf("hgetall %s: %w", hk, err)
	}
	if len(vals) == 0 {
		return LocationRecord{}, fmt.Errorf("no location cached for %s %s", userType, string(id))
	}

	lat, _ := strconv.ParseFloat(vals["lat"], 64)
	lng, _ := strconv.ParseFloat(vals["lng"], 64)
	ts, _ := strconv.ParseInt(vals["updated_at"], 10, 64)

	return LocationRecord{
		UserID:    id,
		Position:  types.Point{Lat: lat, Lng: lng},
		UpdatedAt: time.Unix(ts, 0),
	}, nil
}

// NearbyDrivers returns driver IDs within radiusKm of point p, ordered by
// distance ascending.
func (s *Store) NearbyDrivers(ctx context.Context, p types.Point, radiusKm float64) ([]types.ID, error) {
	return s.geoSearch(ctx, DriverLocationsKey, p, radiusKm)
}

// NearbyPassengers returns passenger IDs within radiusKm of point p, ordered
// by distance ascending.
func (s *Store) NearbyPassengers(ctx context.Context, p types.Point, radiusKm float64) ([]types.ID, error) {
	return s.geoSearch(ctx, PassengerLocationsKey, p, radiusKm)
}

func (s *Store) geoSearch(ctx context.Context, key string, p types.Point, radiusKm float64) ([]types.ID, error) {
	names, err := s.redis.GeoSearch(ctx, key, &redis.GeoSearchQuery{
		Longitude:  p.Lng,
		Latitude:   p.Lat,
		Radius:     radiusKm,
		RadiusUnit: "km",
		Sort:       "ASC",
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("geosearch %s: %w", key, err)
	}

	ids := make([]types.ID, len(names))
	for i, n := range names {
		ids[i] = types.ID(n)
	}
	return ids, nil
}

// AppendSnapshot upserts the latest location snapshot for a user to Postgres.
// On conflict for the same (user_id, user_type) pair the row is updated in-place.
// See migrations/0007_location_snapshots.sql for the expected schema.
func (s *Store) AppendSnapshot(ctx context.Context, snap Snapshot) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO location_snapshots (user_id, user_type, lat, lng, recorded_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id, user_type)
		 DO UPDATE SET lat = EXCLUDED.lat, lng = EXCLUDED.lng, recorded_at = EXCLUDED.recorded_at`,
		string(snap.UserID), snap.UserType,
		snap.Position.Lat, snap.Position.Lng,
		snap.RecordedAt,
	)
	return err
}
