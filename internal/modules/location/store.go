// README: Location store backed by Redis GEO and Postgres snapshots.
package location

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Store struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

func NewStore(db *pgxpool.Pool, redis *redis.Client) *Store {
	return &Store{db: db, redis: redis}
}

// SetGeo writes a member's coordinates to a Redis GEO key.
func (s *Store) SetGeo(ctx context.Context, key string, lat, lng float64, member string) error {
	return s.redis.GeoAdd(ctx, key, &redis.GeoLocation{
		Name:      member,
		Latitude:  lat,
		Longitude: lng,
	}).Err()
}

// SaveMetadata stores location metadata as JSON in Redis.
func (s *Store) SaveMetadata(ctx context.Context, key string, meta LocationMetadata) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return s.redis.Set(ctx, key, data, 0).Err()
}

// GetMetadata retrieves location metadata from Redis. Returns nil if the key does not exist.
func (s *Store) GetMetadata(ctx context.Context, key string) (*LocationMetadata, error) {
	data, err := s.redis.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var meta LocationMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// SetTTL sets the expiration on a Redis key.
func (s *Store) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	return s.redis.Expire(ctx, key, ttl).Err()
}

// AppendSnapshot persists a location snapshot to Postgres (future phase).
func (s *Store) AppendSnapshot(ctx context.Context, snap Snapshot) error {
	return errors.New("not implemented")
}
