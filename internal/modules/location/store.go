// README: Location store backed by Redis GEO, Postgres snapshots, and Firebase RTDB (read-only).
package location

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/db"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/api/option"

	"ark/internal/types"
)

const (
	geoKeyDrivers    = "geo:drivers"
	geoKeyPassengers = "geo:passengers"
	statusTTL        = 60 * time.Second
)

type Store struct {
	db       *pgxpool.Pool
	redis    *redis.Client
	fbApp    *firebase.App
	dbClient *db.Client
}

// NewStore initialises the location store with Firebase RTDB (for polling) and Redis GEO.
// firebaseCredsJSON is the raw Firebase service-account JSON (from FIREBASE_CREDENTIALS_JSON env var).
func NewStore(ctx context.Context, dbPool *pgxpool.Pool, redisClient *redis.Client, firebaseCredsJSON []byte) (*Store, error) {
	projectID, err := parseProjectIDFromJSON(firebaseCredsJSON)
	if err != nil {
		return nil, fmt.Errorf("parsing firebase credentials: %w", err)
	}

	databaseURL := fmt.Sprintf("https://%s-default-rtdb.asia-southeast1.firebasedatabase.app", projectID)

	conf := &firebase.Config{DatabaseURL: databaseURL}
	app, err := firebase.NewApp(ctx, conf, option.WithCredentialsJSON(firebaseCredsJSON))
	if err != nil {
		return nil, fmt.Errorf("initialising firebase app: %w", err)
	}

	dbClient, err := app.Database(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialising firebase RTDB client: %w", err)
	}

	return &Store{
		db:       dbPool,
		redis:    redisClient,
		fbApp:    app,
		dbClient: dbClient,
	}, nil
}

func parseProjectIDFromJSON(data []byte) (string, error) {
	var sa struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(data, &sa); err != nil {
		return "", fmt.Errorf("parsing firebase credentials JSON: %w", err)
	}
	if sa.ProjectID == "" {
		return "", fmt.Errorf("project_id is empty in firebase credentials JSON")
	}
	return sa.ProjectID, nil
}

// ---------------------------------------------------------------------------
// Redis GEO helpers
// ---------------------------------------------------------------------------

func geoSetKey(userType string) string {
	if userType == "passenger" {
		return geoKeyPassengers
	}
	return geoKeyDrivers
}

func statusKey(userType string, id types.ID) string {
	return userType + "_status:" + string(id)
}

// SetGeo writes a batch of user positions into the Redis GEO sorted set and
// refreshes each status key with a 60-second TTL, all in a single pipeline.
func (s *Store) SetGeo(ctx context.Context, entries []GeoEntry, userType string) error {
	if len(entries) == 0 {
		return nil
	}

	geoMembers := make([]*redis.GeoLocation, len(entries))
	for i, e := range entries {
		geoMembers[i] = &redis.GeoLocation{
			Name:      string(e.ID),
			Longitude: e.Pos.Lng,
			Latitude:  e.Pos.Lat,
		}
	}

	pipe := s.redis.Pipeline()
	pipe.GeoAdd(ctx, geoSetKey(userType), geoMembers...)
	for _, e := range entries {
		pipe.Set(ctx, statusKey(userType, e.ID), "1", statusTTL)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("SetGeo batch %s (%d entries): %w", userType, len(entries), err)
	}
	return nil
}

// GetNearbyUsersFromRedis performs a GEOSEARCH for users within radiusKm of
// (lat, lng) and filters out any whose status key has expired (offline).
// Expired members are removed from the GEO set asynchronously (lazy deletion).
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

	// Lazy deletion: remove stale GEO members in the background so the set
	// does not grow unboundedly.
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

// ---------------------------------------------------------------------------
// Firebase RTDB read (used by the background poller)
// ---------------------------------------------------------------------------

// rtdbUserEntry mirrors a user entry stored in Firebase RTDB.
type rtdbUserEntry struct {
	Lat    float64 `json:"lat"`
	Lng    float64 `json:"lng"`
	Status string  `json:"status"`
}

// rtdbNodeAndStatus returns the RTDB node path and the active status string
// for a given user type.
func rtdbNodeAndStatus(userType string) (node, activeStatus string) {
	if userType == "passenger" {
		return "passenger_locations", "looking_for_ride"
	}
	return "driver_locations", "online"
}

// FetchActiveUsersFromRTDB reads all currently active users of the given type
// from Firebase RTDB and returns them as GeoEntry slices ready for SetGeo.
func (s *Store) FetchActiveUsersFromRTDB(ctx context.Context, userType string) ([]GeoEntry, error) {
	node, activeStatus := rtdbNodeAndStatus(userType)
	ref := s.dbClient.NewRef(node)

	var data map[string]rtdbUserEntry
	if err := ref.OrderByChild("status").EqualTo(activeStatus).Get(ctx, &data); err != nil {
		return nil, fmt.Errorf("RTDB fetch %s: %w", userType, err)
	}

	entries := make([]GeoEntry, 0, len(data))
	for id, e := range data {
		entries = append(entries, GeoEntry{
			ID:  types.ID(id),
			Pos: types.Point{Lat: e.Lat, Lng: e.Lng},
		})
	}
	return entries, nil
}

// ---------------------------------------------------------------------------
// Postgres
// ---------------------------------------------------------------------------

func (s *Store) AppendSnapshot(ctx context.Context, snap Snapshot) error {
	return errors.New("not implemented")
}
