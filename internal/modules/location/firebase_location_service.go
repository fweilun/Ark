// Package location provides Firebase-based driver/passenger location querying
// and push notifications for the ride-hailing MVP.
package location

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/db"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	"ark/internal/types"
)

// Default credential file located at the project root.
const defaultCredentialsFile = "zoozoo-v1-firebase-adminsdk-fbsvc-2adb5592ce.json"

// FirebaseService provides driver/passenger location queries via RTDB and push
// notifications via FCM. It is fully decoupled from the Order module.
type FirebaseService struct {
	app       *firebase.App
	dbClient  *db.Client
	msgClient *messaging.Client
}

// NewFirebaseService initialises the Firebase Admin SDK by automatically
// loading the service-account key from the project root and deriving the
// RTDB URL for the Southeast Asia region.
func NewFirebaseService(ctx context.Context) (*FirebaseService, error) {
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

	return &FirebaseService{
		app:       app,
		dbClient:  dbClient,
		msgClient: msgClient,
	}, nil
}

// parseProjectID reads the service-account JSON and extracts the project_id.
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
// RTDB data models
// ---------------------------------------------------------------------------

// rtdbDriverEntry mirrors a single driver entry stored in Firebase RTDB
// under the /locations node.
type rtdbDriverEntry struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Status    string  `json:"status"`
	Timestamp int64   `json:"timestamp"`
}

// rtdbPassengerEntry mirrors a single passenger entry stored in Firebase RTDB
// under the /passenger_locations node.
type rtdbPassengerEntry struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Status    string  `json:"status"`
	Timestamp int64   `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Public result types
// ---------------------------------------------------------------------------

// DriverLocation represents a driver's position with computed distance.
type DriverLocation struct {
	DriverID types.ID
	Lat      float64
	Lng      float64
	Distance float64 // km from the queried origin
}

// PassengerLocation represents a passenger's position with computed distance.
type PassengerLocation struct {
	PassengerID types.ID
	Lat         float64
	Lng         float64
	Distance    float64
	Status      string
}

// OrderInfo contains the payload data to send via FCM.
type OrderInfo struct {
	OrderID      types.ID
	PickupLat    float64
	PickupLng    float64
	DropoffLat   float64
	DropoffLng   float64
	EstimatedFee float64
}

// ---------------------------------------------------------------------------
// Internal RTDB helpers
// ---------------------------------------------------------------------------

// queryActiveDrivers fetches only drivers with status "online" from the
// /locations node using an ordered query.
func (s *FirebaseService) queryActiveDrivers(ctx context.Context) (map[string]rtdbDriverEntry, error) {
	ref := s.dbClient.NewRef("driver_locations")

	var data map[string]rtdbDriverEntry
	if err := ref.OrderByChild("status").EqualTo("online").Get(ctx, &data); err != nil {
		return nil, fmt.Errorf("querying active drivers: %w", err)
	}
	return data, nil
}

// queryActivePassengers fetches only passengers with status "looking_for_ride"
// from the /passenger_locations node.
func (s *FirebaseService) queryActivePassengers(ctx context.Context) (map[string]rtdbPassengerEntry, error) {
	ref := s.dbClient.NewRef("passenger_locations")

	var data map[string]rtdbPassengerEntry
	if err := ref.OrderByChild("status").EqualTo("looking_for_ride").Get(ctx, &data); err != nil {
		return nil, fmt.Errorf("querying active passengers: %w", err)
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// GetNearbyDrivers queries Firebase RTDB for online drivers within radiusKm
// of the given origin, sorted by distance ascending (closest first).
func (s *FirebaseService) GetNearbyDrivers(ctx context.Context, lat, lng, radiusKm float64) ([]DriverLocation, error) {
	data, err := s.queryActiveDrivers(ctx)
	if err != nil {
		return nil, err
	}

	var result []DriverLocation
	for driverID, entry := range data {
		dist := haversineKm(lat, lng, entry.Lat, entry.Lng)
		if dist <= radiusKm {
			result = append(result, DriverLocation{
				DriverID: types.ID(driverID),
				Lat:      entry.Lat,
				Lng:      entry.Lng,
				Distance: dist,
			})
		}
	}

	sortByDistance(result, func(d DriverLocation) float64 { return d.Distance })
	return result, nil
}

// GetNearbyPassengers queries the "passenger_locations" RTDB node for
// passengers actively looking for a ride within radiusKm.
//
// Note: For friend tracking, the Flutter app listens directly to
// /passenger_locations/{friendID} in Firebase for O(1) performance.
// This method is for system-level matching and visibility only.
func (s *FirebaseService) GetNearbyPassengers(ctx context.Context, lat, lng, radiusKm float64) ([]PassengerLocation, error) {
	data, err := s.queryActivePassengers(ctx)
	if err != nil {
		return nil, err
	}

	var result []PassengerLocation
	for passengerID, entry := range data {
		dist := haversineKm(lat, lng, entry.Lat, entry.Lng)
		if dist <= radiusKm {
			result = append(result, PassengerLocation{
				PassengerID: types.ID(passengerID),
				Lat:         entry.Lat,
				Lng:         entry.Lng,
				Distance:    dist,
				Status:      entry.Status,
			})
		}
	}

	sortByDistance(result, func(p PassengerLocation) float64 { return p.Distance })
	return result, nil
}

// NotifyDriverNewOrder sends an FCM data message to the specified driver's
// device. The deviceToken must be resolved by the caller.
func (s *FirebaseService) NotifyDriverNewOrder(ctx context.Context, deviceToken string, info OrderInfo) error {
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
