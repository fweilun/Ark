// README: Location service handles high-frequency updates, geo calculations, and Firebase querying.
package location

import (
	"context"
	"errors"
	"math"
	"time"

	"ark/internal/types"
)

const earthRadiusKm = 6371.0

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

func (s *Service) Update(ctx context.Context, u Update) error {
	return errors.New("not implemented")
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

// GetNearbyDrivers queries Firebase RTDB for online drivers within radiusKm
// of the given origin, sorted by distance ascending (closest first).
func (s *Service) GetNearbyDrivers(ctx context.Context, lat, lng, radiusKm float64) ([]DriverLocation, error) {
	data, err := s.store.queryActiveDrivers(ctx)
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
func (s *Service) GetNearbyPassengers(ctx context.Context, lat, lng, radiusKm float64) ([]PassengerLocation, error) {
	data, err := s.store.queryActivePassengers(ctx)
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

// NotifyDriverNewOrder delegates to the store to send an FCM data message.
func (s *Service) NotifyDriverNewOrder(ctx context.Context, deviceToken string, info OrderInfo) error {
	return s.store.NotifyDriverNewOrder(ctx, deviceToken, info)
}

// ---------------------------------------------------------------------------
// Geo calculations and sorting
// ---------------------------------------------------------------------------

// haversineKm returns the great-circle distance in kilometres between two
// points specified in decimal degrees.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	dLat := degreesToRadians(lat2 - lat1)
	dLng := degreesToRadians(lng2 - lng1)

	rLat1 := degreesToRadians(lat1)
	rLat2 := degreesToRadians(lat2)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rLat1)*math.Cos(rLat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180.0
}

// sortByDistance performs an insertion sort (fine for small N) on any slice
// where each element exposes a distance via the accessor function.
func sortByDistance[T any](items []T, dist func(T) float64) {
	for i := 1; i < len(items); i++ {
		key := items[i]
		j := i - 1
		for j >= 0 && dist(items[j]) > dist(key) {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = key
	}
}
