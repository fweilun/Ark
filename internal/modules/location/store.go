// README: Location store backed by Redis GEO and Postgres snapshots.
package location

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"ark/internal/types"
)

const (
	geoDriversKey    = "geo:drivers"
	geoPassengersKey = "geo:passengers"
)

type Store struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

func NewStore(db *pgxpool.Pool, redis *redis.Client) *Store {
	return &Store{db: db, redis: redis}
}

// SetGeo stores the user's position in the appropriate Redis GEO set.
func (s *Store) SetGeo(ctx context.Context, id types.ID, pos types.Point, userType string) error {
	key := geoDriversKey
	if userType != "driver" {
		key = geoPassengersKey
	}
	return s.redis.GeoAdd(ctx, key, &redis.GeoLocation{
		Name:      string(id),
		Longitude: pos.Lng,
		Latitude:  pos.Lat,
	}).Err()
}

// AppendSnapshot persists a location snapshot to the Postgres snapshots table.
func (s *Store) AppendSnapshot(ctx context.Context, snap Snapshot) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO location_snapshots (user_id, user_type, lat, lng, recorded_at) VALUES ($1, $2, $3, $4, $5)`,
		string(snap.UserID), snap.UserType, snap.Position.Lat, snap.Position.Lng, snap.RecordedAt,
	)
	return err
}
