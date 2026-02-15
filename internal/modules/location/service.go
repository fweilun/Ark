// README: Location service handles high-frequency updates with validation, throttling, and idempotency.
package location

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	geoKeyDrivers    = "geo:drivers"
	geoKeyPassengers = "geo:passengers"
	metaTTL          = 300 * time.Second
	throttleMs       = 1000
)

var (
	ErrValidation = errors.New("validation error")
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

// UpdateDriverLocation processes a driver location update.
func (s *Service) UpdateDriverLocation(ctx context.Context, u DriverLocationUpdate) (UpdateResult, error) {
	if err := validateBase(u.LocationUpdate); err != nil {
		return UpdateResult{}, err
	}

	metaKey := fmt.Sprintf("loc:driver:%s", u.UserID)

	existing, err := s.store.GetMetadata(ctx, metaKey)
	if err != nil {
		return UpdateResult{}, err
	}

	if throttled(existing, u.Point.TsMs) {
		return UpdateResult{Accepted: false, Throttled: true}, nil
	}

	if existing != nil && u.Seq <= existing.LastSeq {
		return UpdateResult{Accepted: true, Throttled: false}, nil
	}

	if err := s.store.SetGeo(ctx, geoKeyDrivers, u.Point.Lat, u.Point.Lng, u.UserID); err != nil {
		return UpdateResult{}, err
	}

	meta := LocationMetadata{
		Lat:         u.Point.Lat,
		Lng:         u.Point.Lng,
		TsMs:        u.Point.TsMs,
		LastSeq:     u.Seq,
		IsActive:    u.IsActive,
		DriverState: u.DriverState,
		OrderID:     u.OrderID,
	}
	if err := s.store.SaveMetadata(ctx, metaKey, meta); err != nil {
		return UpdateResult{}, err
	}

	if err := s.store.SetTTL(ctx, metaKey, metaTTL); err != nil {
		return UpdateResult{}, err
	}

	return UpdateResult{Accepted: true, Throttled: false}, nil
}

// UpdatePassengerLocation processes a passenger location update.
func (s *Service) UpdatePassengerLocation(ctx context.Context, u LocationUpdate) (UpdateResult, error) {
	if err := validateBase(u); err != nil {
		return UpdateResult{}, err
	}

	metaKey := fmt.Sprintf("loc:passenger:%s", u.UserID)

	existing, err := s.store.GetMetadata(ctx, metaKey)
	if err != nil {
		return UpdateResult{}, err
	}

	if throttled(existing, u.Point.TsMs) {
		return UpdateResult{Accepted: false, Throttled: true}, nil
	}

	if existing != nil && u.Seq <= existing.LastSeq {
		return UpdateResult{Accepted: true, Throttled: false}, nil
	}

	if err := s.store.SetGeo(ctx, geoKeyPassengers, u.Point.Lat, u.Point.Lng, u.UserID); err != nil {
		return UpdateResult{}, err
	}

	meta := LocationMetadata{
		Lat:      u.Point.Lat,
		Lng:      u.Point.Lng,
		TsMs:     u.Point.TsMs,
		LastSeq:  u.Seq,
		IsActive: u.IsActive,
	}
	if err := s.store.SaveMetadata(ctx, metaKey, meta); err != nil {
		return UpdateResult{}, err
	}

	if err := s.store.SetTTL(ctx, metaKey, metaTTL); err != nil {
		return UpdateResult{}, err
	}

	return UpdateResult{Accepted: true, Throttled: false}, nil
}

func validateBase(u LocationUpdate) error {
	if u.UserID == "" {
		return fmt.Errorf("%w: missing user_id", ErrValidation)
	}
	if u.Seq <= 0 {
		return fmt.Errorf("%w: missing seq", ErrValidation)
	}
	if u.Point.Lat < -90 || u.Point.Lat > 90 {
		return fmt.Errorf("%w: lat out of range", ErrValidation)
	}
	if u.Point.Lng < -180 || u.Point.Lng > 180 {
		return fmt.Errorf("%w: lng out of range", ErrValidation)
	}
	return nil
}

func throttled(existing *LocationMetadata, incomingTsMs int64) bool {
	if existing == nil {
		return false
	}
	return (incomingTsMs - existing.TsMs) < throttleMs
}
