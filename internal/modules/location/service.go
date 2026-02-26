// README: Location service handles high-frequency updates with optional snapshot flushing.
package location

import (
	"context"
	"time"

	"ark/internal/types"
)

// Service orchestrates location updates and queries backed by the Store.
type Service struct {
	store *Store
}

// NewService creates a Service wired to the given Store.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// Update holds the data for a single location update from either a driver or
// a passenger. UserType must be "driver" or "passenger".
type Update struct {
	UserID   types.ID
	UserType string
	Position types.Point
}

// Update stores the user's current position in Redis.
func (s *Service) Update(ctx context.Context, u Update) error {
	return s.store.SetGeo(ctx, u.UserID, u.Position, u.UserType)
}

// FlushSnapshot writes the current location to the Postgres snapshot table.
// Call this at a lower frequency than Update (e.g. every 30 s) to create a
// persistent audit trail without hammering the database.
func (s *Service) FlushSnapshot(ctx context.Context, u Update) error {
	snap := Snapshot{
		UserID:     u.UserID,
		UserType:   u.UserType,
		Position:   u.Position,
		RecordedAt: time.Now(),
	}
	return s.store.AppendSnapshot(ctx, snap)
}

// GetNearbyDrivers returns IDs of drivers whose cached position is within
// radiusKm of pos, ordered closest-first. Backed by Redis GEOSEARCH so the
// query is O(N+log M) and does not touch Firebase.
func (s *Service) GetNearbyDrivers(ctx context.Context, pos types.Point, radiusKm float64) ([]types.ID, error) {
	return s.store.NearbyDrivers(ctx, pos, radiusKm)
}

// GetLocationOfDriver returns the most-recently cached location of a driver.
// Returns an error when no entry exists or the TTL has elapsed (> 5 min old).
func (s *Service) GetLocationOfDriver(ctx context.Context, driverID types.ID) (LocationRecord, error) {
	return s.store.GetLocation(ctx, driverID, "driver")
}

// GetLocationOfPassenger returns the most-recently cached location of a
// passenger. Returns an error when no entry exists or the TTL has elapsed.
func (s *Service) GetLocationOfPassenger(ctx context.Context, passengerID types.ID) (LocationRecord, error) {
	return s.store.GetLocation(ctx, passengerID, "passenger")
}
