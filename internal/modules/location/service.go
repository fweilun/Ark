// README: Location service handles high-frequency updates with optional snapshot flushing.
package location

import (
	"context"
	"fmt"
	"log"
	"time"

	"ark/internal/types"
)

type Service struct {
	store    *Store
	firebase *FirebaseService
}

func NewService(store *Store, firebase *FirebaseService) *Service {
	return &Service{store: store, firebase: firebase}
}

type Update struct {
	UserID   types.ID
	UserType string
	Position types.Point
}

// Update writes the user's position to Redis (for backend geo queries) and to
// Firebase RTDB (for frontend real-time listening). Both writes must succeed.
func (s *Service) Update(ctx context.Context, u Update) error {
	if err := s.store.SetGeo(ctx, u.UserID, u.Position, u.UserType); err != nil {
		log.Printf("location: Redis SetGeo failed for %s %s: %v", u.UserType, u.UserID, err)
		return fmt.Errorf("updating Redis geo: %w", err)
	}
	if err := s.firebase.WriteLocation(ctx, u.UserID, u.Position, u.UserType); err != nil {
		log.Printf("location: Firebase write failed for %s %s: %v", u.UserType, u.UserID, err)
		return fmt.Errorf("updating Firebase location: %w", err)
	}
	return nil
}

func (s *Service) FlushSnapshot(ctx context.Context, u Update) error {
	snap := Snapshot{
		UserID:     u.UserID,
		UserType:   u.UserType,
		Position:   u.Position,
		RecordedAt: time.Now(),
	}
	return s.store.AppendSnapshot(ctx, snap)
}

// GetNearbyDrivers returns online drivers within radiusKm of (lat, lng),
// sorted by distance ascending. Presence is determined by the Redis status TTL.
func (s *Service) GetNearbyDrivers(ctx context.Context, lat, lng, radiusKm float64) ([]DriverLocation, error) {
	users, err := s.store.GetNearbyUsersFromRedis(ctx, lat, lng, radiusKm, "driver")
	if err != nil {
		return nil, err
	}
	result := make([]DriverLocation, len(users))
	for i, u := range users {
		result[i] = DriverLocation{
			DriverID: u.ID,
			Lat:      u.Lat,
			Lng:      u.Lng,
			Distance: u.Distance,
		}
	}
	return result, nil
}

// GetNearbyPassengers returns passengers looking for a ride within radiusKm of
// (lat, lng), sorted by distance ascending.
func (s *Service) GetNearbyPassengers(ctx context.Context, lat, lng, radiusKm float64) ([]PassengerLocation, error) {
	users, err := s.store.GetNearbyUsersFromRedis(ctx, lat, lng, radiusKm, "passenger")
	if err != nil {
		return nil, err
	}
	result := make([]PassengerLocation, len(users))
	for i, u := range users {
		result[i] = PassengerLocation{
			PassengerID: u.ID,
			Lat:         u.Lat,
			Lng:         u.Lng,
			Distance:    u.Distance,
			Status:      "looking_for_ride",
		}
	}
	return result, nil
}

// NotifyDriverNewOrder delegates FCM push notification to the Firebase service.
func (s *Service) NotifyDriverNewOrder(ctx context.Context, deviceToken string, info OrderInfo) error {
	return s.firebase.NotifyDriverNewOrder(ctx, deviceToken, info)
}
