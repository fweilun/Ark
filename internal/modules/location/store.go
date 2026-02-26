// README: Location store backed by Redis GEO, Postgres snapshots, and Firebase RTDB.
package location

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/db"
	"firebase.google.com/go/v4/messaging"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/api/option"

	"ark/internal/types"
)

const (
	defaultCredentialsFile = "zoozoo-v1-firebase-adminsdk-fbsvc-2adb5592ce.json"
	geoKeyDrivers          = "geo:drivers"
	geoKeyPassengers       = "geo:passengers"
	statusTTL              = 60 * time.Second
)

type Store struct {
	db        *pgxpool.Pool
	redis     *redis.Client
	fbApp     *firebase.App
	dbClient  *db.Client
	msgClient *messaging.Client
}

// NewStore initialises the location store, including the Firebase Admin SDK.
// The credentials file is read from the project root.
func NewStore(ctx context.Context, dbPool *pgxpool.Pool, redisClient *redis.Client) (*Store, error) {
	projectID, err := parseProjectID(defaultCredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("reading credentials file: %w", err)
	}

	databaseURL := fmt.Sprintf("https://%s-default-rtdb.asia-southeast1.firebasedatabase.app", projectID)

	conf := &firebase.Config{DatabaseURL: databaseURL}
	app, err := firebase.NewApp(ctx, conf, option.WithCredentialsFile(defaultCredentialsFile))
	if err != nil {
		return nil, fmt.Errorf("initialising firebase app: %w", err)
	}

	dbClient, err := app.Database(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialising firebase RTDB client: %w", err)
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialising firebase messaging client: %w", err)
	}

	return &Store{
		db:        dbPool,
		redis:     redisClient,
		fbApp:     app,
		dbClient:  dbClient,
		msgClient: msgClient,
	}, nil
}

func parseProjectID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	var sa struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(data, &sa); err != nil {
		return "", fmt.Errorf("parsing %s: %w", path, err)
	}
	if sa.ProjectID == "" {
		return "", fmt.Errorf("project_id is empty in %s", path)
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

// SetGeo writes the user's position to the Redis GEO sorted set and refreshes
// their status key with a 60-second TTL in a single pipeline.
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
	// does not grow unboundedly. A detached context prevents cancellation if
	// the request context ends before the goroutine executes.
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
// Firebase RTDB writes
// ---------------------------------------------------------------------------

// WriteLocation writes the user's position to the appropriate Firebase RTDB
// node so the frontend can listen in real time.
func (s *Store) WriteLocation(ctx context.Context, id types.ID, pos types.Point, userType string) error {
	node, status := rtdbNodeAndStatus(userType)
	ref := s.dbClient.NewRef(node + "/" + string(id))
	entry := map[string]interface{}{
		"lat":       pos.Lat,
		"lng":       pos.Lng,
		"status":    status,
		"timestamp": time.Now().UnixMilli(),
	}
	if err := ref.Set(ctx, entry); err != nil {
		return fmt.Errorf("Firebase WriteLocation %s %s: %w", userType, id, err)
	}
	return nil
}

// rtdbNodeAndStatus returns the RTDB node path and the active status string
// for a given user type.
func rtdbNodeAndStatus(userType string) (node, status string) {
	if userType == "passenger" {
		return "passenger_locations", "looking_for_ride"
	}
	return "driver_locations", "online"
}

// ---------------------------------------------------------------------------
// Firebase Cloud Messaging
// ---------------------------------------------------------------------------

// NotifyDriverNewOrder sends an FCM data message to the specified driver's device.
func (s *Store) NotifyDriverNewOrder(ctx context.Context, deviceToken string, info OrderInfo) error {
	if deviceToken == "" {
		return fmt.Errorf("empty device token for order %s", string(info.OrderID))
	}

	msg := &messaging.Message{
		Token: deviceToken,
		Data: map[string]string{
			"type":          "new_order",
			"order_id":      string(info.OrderID),
			"pickup_lat":    strconv.FormatFloat(info.PickupLat, 'f', 6, 64),
			"pickup_lng":    strconv.FormatFloat(info.PickupLng, 'f', 6, 64),
			"dropoff_lat":   strconv.FormatFloat(info.DropoffLat, 'f', 6, 64),
			"dropoff_lng":   strconv.FormatFloat(info.DropoffLng, 'f', 6, 64),
			"estimated_fee": strconv.FormatFloat(info.EstimatedFee, 'f', 2, 64),
		},
		Notification: &messaging.Notification{
			Title: "New ride request",
			Body:  fmt.Sprintf("Pickup nearby — estimated fare $%.2f", info.EstimatedFee),
		},
		Android: &messaging.AndroidConfig{Priority: "high"},
	}

	messageID, err := s.msgClient.Send(ctx, msg)
	if err != nil {
		return fmt.Errorf("sending FCM to token %s: %w", deviceToken, err)
	}

	log.Printf("FCM sent for order %s, message_id=%s", string(info.OrderID), messageID)
	return nil
}

// ---------------------------------------------------------------------------
// Postgres
// ---------------------------------------------------------------------------

func (s *Store) AppendSnapshot(ctx context.Context, snap Snapshot) error {
	return errors.New("not implemented")
}
