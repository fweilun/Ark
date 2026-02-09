// README: Pricing service computes fare estimates.
package pricing

import (
    "context"
    "errors"

    "ark/internal/types"
)

type Service struct {
    store *Store
}

func NewService(store *Store) *Service {
    return &Service{store: store}
}

func (s *Service) Estimate(ctx context.Context, distanceKm float64, rideType string) (types.Money, error) {
    return types.Money{}, errors.New("not implemented")
}
