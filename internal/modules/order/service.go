// README: Order service implements state transitions and persistence.
package order

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "errors"
    "math"
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

var (
    ErrInvalidState = errors.New("invalid state transition")
    ErrNotFound     = errors.New("order not found")
    ErrConflict     = errors.New("order state conflict")
    ErrActiveOrder  = errors.New("passenger has active order")
    ErrBadRequest   = errors.New("bad request")
)

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
    if cmd.PassengerID == "" || cmd.RideType == "" {
        return "", ErrBadRequest
    }
    active, err := s.store.HasActiveByPassenger(ctx, cmd.PassengerID)
    if err != nil {
        return "", err
    }
    if active {
        return "", ErrActiveOrder
    }

    id := newID()
    now := time.Now()
    est := types.Money{Amount: 0, Currency: "TWD"}
    if s.pricing != nil {
        if m, err := s.pricing.Estimate(ctx, distanceKm(cmd.Pickup, cmd.Dropoff), cmd.RideType); err == nil {
            est = m
        }
    }

    o := &Order{
        ID:            id,
        PassengerID:   cmd.PassengerID,
        Status:        StatusCreated,
        StatusVersion: 0,
        Pickup:        cmd.Pickup,
        Dropoff:       cmd.Dropoff,
        RideType:      cmd.RideType,
        EstimatedFee:  est,
        CreatedAt:     now,
    }
    if err := s.store.Create(ctx, o); err != nil {
        return "", err
    }
    _ = s.store.AppendEvent(ctx, &Event{
        OrderID:    id,
        FromStatus: StatusNone,
        ToStatus:   StatusCreated,
        ActorType:  "passenger",
        ActorID:    &cmd.PassengerID,
        CreatedAt:  now,
    })
    return id, nil
}

func (s *Service) Match(ctx context.Context, cmd MatchCommand) error {
    o, err := s.store.Get(ctx, cmd.OrderID)
    if err != nil {
        return err
    }
    if !CanTransition(o.Status, StatusMatched) {
        return ErrInvalidState
    }
    ok, err := s.store.UpdateStatus(ctx, o.ID, o.Status, StatusMatched, o.StatusVersion, &cmd.DriverID)
    if err != nil {
        return err
    }
    if !ok {
        return ErrConflict
    }
    _ = s.store.AppendEvent(ctx, &Event{
        OrderID:    o.ID,
        FromStatus: StatusCreated,
        ToStatus:   StatusMatched,
        ActorType:  "system",
        ActorID:    nil,
        CreatedAt:  time.Now(),
    })
    return nil
}

func (s *Service) Accept(ctx context.Context, cmd AcceptCommand) error {
    o, err := s.store.Get(ctx, cmd.OrderID)
    if err != nil {
        return err
    }
    if !CanTransition(o.Status, StatusAccepted) {
        return ErrInvalidState
    }
    ok, err := s.store.UpdateStatus(ctx, o.ID, o.Status, StatusAccepted, o.StatusVersion, &cmd.DriverID)
    if err != nil {
        return err
    }
    if !ok {
        return ErrConflict
    }
    _ = s.store.AppendEvent(ctx, &Event{
        OrderID:    o.ID,
        FromStatus: StatusMatched,
        ToStatus:   StatusAccepted,
        ActorType:  "driver",
        ActorID:    &cmd.DriverID,
        CreatedAt:  time.Now(),
    })
    return nil
}

func (s *Service) Start(ctx context.Context, cmd StartCommand) error {
    o, err := s.store.Get(ctx, cmd.OrderID)
    if err != nil {
        return err
    }
    if !CanTransition(o.Status, StatusInProgress) {
        return ErrInvalidState
    }
    ok, err := s.store.UpdateStatus(ctx, o.ID, o.Status, StatusInProgress, o.StatusVersion, o.DriverID)
    if err != nil {
        return err
    }
    if !ok {
        return ErrConflict
    }
    _ = s.store.AppendEvent(ctx, &Event{
        OrderID:    o.ID,
        FromStatus: StatusAccepted,
        ToStatus:   StatusInProgress,
        ActorType:  "driver",
        ActorID:    o.DriverID,
        CreatedAt:  time.Now(),
    })
    return nil
}

func (s *Service) Complete(ctx context.Context, cmd CompleteCommand) error {
    o, err := s.store.Get(ctx, cmd.OrderID)
    if err != nil {
        return err
    }
    if !CanTransition(o.Status, StatusCompleted) {
        return ErrInvalidState
    }
    ok, err := s.store.UpdateStatus(ctx, o.ID, o.Status, StatusCompleted, o.StatusVersion, o.DriverID)
    if err != nil {
        return err
    }
    if !ok {
        return ErrConflict
    }
    _ = s.store.AppendEvent(ctx, &Event{
        OrderID:    o.ID,
        FromStatus: StatusInProgress,
        ToStatus:   StatusCompleted,
        ActorType:  "driver",
        ActorID:    o.DriverID,
        CreatedAt:  time.Now(),
    })
    return nil
}

func (s *Service) Cancel(ctx context.Context, cmd CancelCommand) error {
    o, err := s.store.Get(ctx, cmd.OrderID)
    if err != nil {
        return err
    }
    if !CanTransition(o.Status, StatusCancelled) {
        return ErrInvalidState
    }
    ok, err := s.store.UpdateStatus(ctx, o.ID, o.Status, StatusCancelled, o.StatusVersion, o.DriverID)
    if err != nil {
        return err
    }
    if !ok {
        return ErrConflict
    }
    actorID := o.DriverID
    if cmd.ActorType == "passenger" {
        actorID = &o.PassengerID
    }
    _ = s.store.AppendEvent(ctx, &Event{
        OrderID:    o.ID,
        FromStatus: o.Status,
        ToStatus:   StatusCancelled,
        ActorType:  cmd.ActorType,
        ActorID:    actorID,
        CreatedAt:  time.Now(),
    })
    return nil
}

func (s *Service) Get(ctx context.Context, id types.ID) (*Order, error) {
    return s.store.Get(ctx, id)
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

func newID() types.ID {
    var b [16]byte
    _, _ = rand.Read(b[:])
    return types.ID(hex.EncodeToString(b[:]))
}

func distanceKm(a, b types.Point) float64 {
    const R = 6371.0
    lat1 := a.Lat * math.Pi / 180.0
    lat2 := b.Lat * math.Pi / 180.0
    dlat := (b.Lat - a.Lat) * math.Pi / 180.0
    dlng := (b.Lng - a.Lng) * math.Pi / 180.0
    h := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dlng/2)*math.Sin(dlng/2)
    return 2 * R * math.Asin(math.Sqrt(h))
}
