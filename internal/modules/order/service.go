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

// TODO(schedule): Add scheduled-order commands and service methods.
// - CreateScheduled, ListScheduledByPassenger, ListAvailableScheduled
// - ClaimScheduled, CancelScheduledByPassenger, CancelScheduledByDriver
// - RunScheduleIncentiveTicker, RunScheduleExpireTicker
// Ensure state transitions are enforced for scheduled vs realtime flows.

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

type ArriveCommand struct {
	OrderID types.ID
}

type MeetCommand struct {
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

type DenyCommand struct {
	OrderID  types.ID
	DriverID types.ID
}

type PayCommand struct {
	OrderID types.ID
}

// --- State flow helpers (kept separate from service methods) ---

type transitionParams struct {
	to        Status
	driverID  *types.ID
	actorType string
	actorID   *types.ID
}

func (s *Service) applyTransition(ctx context.Context, orderID types.ID, p transitionParams) error {
	o, err := s.store.Get(ctx, orderID)
	if err != nil {
		return err
	}
	if !CanTransition(o.Status, p.to) {
		return ErrInvalidState
	}
	ok, err := s.store.UpdateStatus(ctx, o.ID, o.Status, p.to, o.StatusVersion, p.driverID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrConflict
	}
	actorID := resolveActorID(o, p)
	_ = s.store.AppendEvent(ctx, &Event{
		OrderID:    o.ID,
		FromStatus: o.Status,
		ToStatus:   p.to,
		ActorType:  p.actorType,
		ActorID:    actorID,
		CreatedAt:  time.Now(),
	})
	return nil
}

func resolveActorID(o *Order, p transitionParams) *types.ID {
	if p.actorID != nil {
		return p.actorID
	}
	switch p.actorType {
	case "passenger":
		return &o.PassengerID
	case "driver":
		if p.driverID != nil {
			return p.driverID
		}
		return o.DriverID
	default:
		return nil
	}
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
		Status:        StatusWaiting,
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
		ToStatus:   StatusWaiting,
		ActorType:  "passenger",
		ActorID:    &cmd.PassengerID,
		CreatedAt:  now,
	})
	return id, nil
}

func (s *Service) Match(ctx context.Context, cmd MatchCommand) error {
	return s.applyTransition(ctx, cmd.OrderID, transitionParams{
		to:        StatusApproaching,
		driverID:  &cmd.DriverID,
		actorType: "system",
	})
}

func (s *Service) Accept(ctx context.Context, cmd AcceptCommand) error {
	return s.applyTransition(ctx, cmd.OrderID, transitionParams{
		to:        StatusApproaching,
		driverID:  &cmd.DriverID,
		actorType: "driver",
	})
}

func (s *Service) Start(ctx context.Context, cmd StartCommand) error {
	return s.Meet(ctx, MeetCommand{OrderID: cmd.OrderID})
}

func (s *Service) Arrive(ctx context.Context, cmd ArriveCommand) error {
	return s.applyTransition(ctx, cmd.OrderID, transitionParams{
		to:        StatusArrived,
		actorType: "driver",
	})
}

func (s *Service) Meet(ctx context.Context, cmd MeetCommand) error {
	return s.applyTransition(ctx, cmd.OrderID, transitionParams{
		to:        StatusDriving,
		actorType: "driver",
	})
}

func (s *Service) Complete(ctx context.Context, cmd CompleteCommand) error {
	return s.applyTransition(ctx, cmd.OrderID, transitionParams{
		to:        StatusPayment,
		actorType: "driver",
	})
}

func (s *Service) Cancel(ctx context.Context, cmd CancelCommand) error {
	return s.applyTransition(ctx, cmd.OrderID, transitionParams{
		to:        StatusCancelled,
		actorType: cmd.ActorType,
	})
}

func (s *Service) Get(ctx context.Context, id types.ID) (*Order, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) Deny(ctx context.Context, cmd DenyCommand) error {
	return s.applyTransition(ctx, cmd.OrderID, transitionParams{
		to:        StatusDenied,
		driverID:  &cmd.DriverID,
		actorType: "driver",
	})
}

func (s *Service) Pay(ctx context.Context, cmd PayCommand) error {
	return s.applyTransition(ctx, cmd.OrderID, transitionParams{
		to:        StatusComplete,
		actorType: "system",
	})
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
