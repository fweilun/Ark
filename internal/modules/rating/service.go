// README: Rating service implements CRUD for trip ratings.
package rating

import (
	"context"
	"errors"
)

var (
	ErrNotFound   = errors.New("rating not found")
	ErrBadRequest = errors.New("bad request")
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

type CreateCommand struct {
	TripID       string
	RiderRating  int
	DriverRating int
	Comments     string
}

func isValidRating(r int) bool {
	return r >= 1 && r <= 5
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (int64, error) {
	if cmd.TripID == "" {
		return 0, ErrBadRequest
	}
	if cmd.RiderRating != 0 && !isValidRating(cmd.RiderRating) {
		return 0, ErrBadRequest
	}
	if cmd.DriverRating != 0 && !isValidRating(cmd.DriverRating) {
		return 0, ErrBadRequest
	}
	return s.store.Create(ctx, &Rating{
		TripID:       cmd.TripID,
		RiderRating:  cmd.RiderRating,
		DriverRating: cmd.DriverRating,
		Comments:     cmd.Comments,
	})
}

func (s *Service) Get(ctx context.Context, ratingID int64) (*Rating, error) {
	return s.store.Get(ctx, ratingID)
}

func (s *Service) GetByTrip(ctx context.Context, tripID string) (*Rating, error) {
	if tripID == "" {
		return nil, ErrBadRequest
	}
	return s.store.GetByTrip(ctx, tripID)
}
