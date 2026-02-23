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

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/db"
	"firebase.google.com/go/v4/messaging"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/api/option"

	"ark/internal/types"
)

const defaultCredentialsFile = "zoozoo-v1-firebase-adminsdk-fbsvc-2adb5592ce.json"

type Store struct {
	db        *pgxpool.Pool
	redis     *redis.Client
	fbApp     *firebase.App
	dbClient  *db.Client
	msgClient *messaging.Client
}

// NewStore initialises the Location store, including the Firebase Admin SDK
// connection using the credentials file at the project root.
func NewStore(ctx context.Context, dbPool *pgxpool.Pool, redisClient *redis.Client) (*Store, error) {
	projectID, err := parseProjectID(defaultCredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("reading credentials file: %w", err)
	}

	databaseURL := fmt.Sprintf("https://%s-default-rtdb.asia-southeast1.firebasedatabase.app", projectID)

	conf := &firebase.Config{
		DatabaseURL: databaseURL,
	}
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

func (s *Store) SetGeo(ctx context.Context, id types.ID, pos types.Point, userType string) error {
	return errors.New("not implemented")
}

func (s *Store) AppendSnapshot(ctx context.Context, snap Snapshot) error {
	return errors.New("not implemented")
}

// queryActiveDrivers fetches only drivers with status "online" from the
// /driver_locations node using an ordered query.
func (s *Store) queryActiveDrivers(ctx context.Context) (map[string]rtdbDriverEntry, error) {
	ref := s.dbClient.NewRef("driver_locations")

	var data map[string]rtdbDriverEntry
	if err := ref.OrderByChild("status").EqualTo("online").Get(ctx, &data); err != nil {
		return nil, fmt.Errorf("querying active drivers: %w", err)
	}
	return data, nil
}

// queryActivePassengers fetches only passengers with status "looking_for_ride"
// from the /passenger_locations node.
func (s *Store) queryActivePassengers(ctx context.Context) (map[string]rtdbPassengerEntry, error) {
	ref := s.dbClient.NewRef("passenger_locations")

	var data map[string]rtdbPassengerEntry
	if err := ref.OrderByChild("status").EqualTo("looking_for_ride").Get(ctx, &data); err != nil {
		return nil, fmt.Errorf("querying active passengers: %w", err)
	}
	return data, nil
}

// NotifyDriverNewOrder sends an FCM data message to the specified driver's
// device. The deviceToken must be resolved by the caller.
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
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
	}

	messageID, err := s.msgClient.Send(ctx, msg)
	if err != nil {
		return fmt.Errorf("sending FCM to token %s: %w", deviceToken, err)
	}

	log.Printf("FCM sent for order %s, message_id=%s", string(info.OrderID), messageID)
	return nil
}
