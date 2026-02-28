// README: Driver service — business logic for driver profile, status, and rating.
package driver

import (
	"context"
	"time"

	"ark/internal/http/middleware"
	"ark/internal/types"
)

// Service implements driver-specific business operations.
type Service struct {
	store DriverStore
}

func NewService(store DriverStore) *Service {
	return &Service{store: store}
}

// Create registers a new driver profile. The driver_id is obtained from the request context
// (set by the Auth middleware); no explicit driver_id is accepted in the request body.
func (s *Service) Create(ctx context.Context, licenseNumber string) (*Driver, error) {
	driverID, ok := userIDFromCtx(ctx)
	if !ok {
		return nil, ErrForbidden
	}
	if licenseNumber == "" {
		return nil, ErrBadRequest
	}
	d := &Driver{
		ID:            driverID,
		LicenseNumber: licenseNumber,
		Rating:        5.0,
		Status:        StatusAvailable,
		OnboardedAt:   time.Now(),
	}
	if err := s.store.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// UpdateRating sets a driver's average rating. Called by the Rating module with an explicit driver_id.
func (s *Service) UpdateRating(ctx context.Context, driverID types.ID, newRating float64) error {
	if newRating < 0 || newRating > 5 {
		return ErrBadRequest
	}
	return s.store.UpdateRating(ctx, driverID, newRating)
}

// UpdateStatus updates the authenticated driver's status using driver_id from the request context.
// The update is protected by a row-level lock to prevent concurrent conflicting writes.
func (s *Service) UpdateStatus(ctx context.Context, newStatus string) error {
	driverID, ok := userIDFromCtx(ctx)
	if !ok {
		return ErrForbidden
	}
	if !isValidStatus(newStatus) {
		return ErrBadRequest
	}
	return s.store.UpdateStatusWithLock(ctx, driverID, newStatus)
}

// DriverInfo returns a driver's profile by explicit driver_id. Called by the Order module.
func (s *Service) DriverInfo(ctx context.Context, driverID types.ID) (*Driver, error) {
	return s.store.Get(ctx, driverID)
}

// userIDFromCtx extracts the authenticated user's ID from the Go request context.
// Returns ("", false) if the context carries no user_id (unauthenticated request).
func userIDFromCtx(ctx context.Context) (types.ID, bool) {
	id, ok := middleware.UserIDFromContext(ctx)
	if !ok || id == "" {
		return "", false
	}
	return types.ID(id), true
}

func isValidStatus(s string) bool {
	return s == StatusAvailable || s == StatusOnTrip || s == StatusOffline
}
