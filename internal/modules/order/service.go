// README: Order service implements state transitions and persistence.
package order

import (
    "context"
    "errors"
    "time"

    "ark/internal/types"
)

type Pricing interface {
    Estimate(ctx context.Context, distanceKm float64, rideType string) (types.Money, error)
}

type Service struct {
    store   *Store
    pricing Pricing
}

func NewService(store *Store, pricing Pricing) *Service {
    return &Service{store: store, pricing: pricing}
}

type CreateCommand struct {
    PassengerID types.ID
    Pickup      types.Point
    Dropoff     types.Point
    RideType    string
}

type MatchCommand struct {
    OrderID   types.ID
    DriverID  types.ID
    MatchedAt time.Time
}

type AcceptCommand struct {
    OrderID  types.ID
    DriverID types.ID
}

type StartCommand struct {
    OrderID types.ID
}

type CompleteCommand struct {
    OrderID types.ID
}

type CancelCommand struct {
    OrderID   types.ID
    ActorType string
    Reason    string
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (types.ID, error) {
    return "", errors.New("not implemented")
}

func (s *Service) Match(ctx context.Context, cmd MatchCommand) error {
    return errors.New("not implemented")
}

func (s *Service) Accept(ctx context.Context, cmd AcceptCommand) error {
    return errors.New("not implemented")
}

func (s *Service) Start(ctx context.Context, cmd StartCommand) error {
    return errors.New("not implemented")
}

func (s *Service) Complete(ctx context.Context, cmd CompleteCommand) error {
    return errors.New("not implemented")
}

func (s *Service) Cancel(ctx context.Context, cmd CancelCommand) error {
    return errors.New("not implemented")
}

func (s *Service) Get(ctx context.Context, id types.ID) (*Order, error) {
    return nil, errors.New("not implemented")
}

func (s *Service) RunTimeoutMonitor(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // TODO: query timeout orders and update status
        }
    }
}
