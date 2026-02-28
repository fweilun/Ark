// README: Pricing service computes fare estimates.
package pricing

import (
	"context"

	"ark/internal/types"
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

func (s *Service) Estimate(ctx context.Context, distanceKm float64, rideType string) (types.Money, error) {
	// TODO: implement real pricing from DB
	return types.Money{Amount: 15000, Currency: "TWD"}, nil
}
