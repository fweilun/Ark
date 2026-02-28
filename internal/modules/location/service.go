// README: Location service owns the RTDB→Redis sync poller and geo query helpers.
package location

import (
	"context"
	"log"
	"time"
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

// RunRTDBPoller periodically fetches active user positions from Firebase RTDB
// and syncs them into the Redis GEO index. It blocks until ctx is cancelled.
func (s *Service) RunRTDBPoller(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Printf("location: RTDB poller started (interval=%s)", interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("location: RTDB poller stopped")
			return
		case <-ticker.C:
			s.syncRTDBToRedis(ctx)
		}
	}
}

// syncRTDBToRedis is called on each tick; errors are logged and do not stop
// the poller so a transient RTDB hiccup doesn't break geo queries.
func (s *Service) syncRTDBToRedis(ctx context.Context) {
	for _, userType := range []string{"driver", "passenger"} {
		entries, err := s.store.FetchActiveUsersFromRTDB(ctx, userType)
		if err != nil {
			log.Printf("location: poller fetch %s from RTDB: %v", userType, err)
			continue
		}
		if len(entries) == 0 {
			continue
		}
		if err := s.store.SetGeo(ctx, entries, userType); err != nil {
			log.Printf("location: poller sync %s to Redis: %v", userType, err)
			continue
		}
		log.Printf("location: poller synced %d %ss from RTDB to Redis", len(entries), userType)
	}
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
