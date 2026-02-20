// README: Scheduled-order workflow implementation.
package order

import (
	"context"
	"time"

	"ark/internal/types"
)

var (
	// ErrScheduleConflict is returned when a scheduled order cannot be claimed (already taken).
	ErrScheduleConflict = ErrConflict
)

const (
	// driverCancelBonusIncrement is added to incentive_bonus when a driver cancels a claimed order.
	driverCancelBonusIncrement int64 = 50
	// incentiveTickerBump is the amount added to incentive_bonus each tick for approaching orders.
	incentiveTickerBump int64 = 25
	// incentiveTickerInterval controls how often the incentive ticker fires.
	incentiveTickerInterval = 5 * time.Minute
	// expireTickerInterval controls how often the expiry ticker fires.
	expireTickerInterval = 1 * time.Minute
)

// CreateScheduledCommand holds the fields required to create a scheduled order.
type CreateScheduledCommand struct {
	PassengerID        types.ID
	Pickup             types.Point
	Dropoff            types.Point
	RideType           string
	ScheduledAt        time.Time
	ScheduleWindowMins int
}

// ClaimScheduledCommand is used by a driver to claim (accept) a scheduled order.
type ClaimScheduledCommand struct {
	OrderID  types.ID
	DriverID types.ID
}

// CancelScheduledCommand is used by the passenger to cancel a scheduled order.
type CancelScheduledCommand struct {
	OrderID   types.ID
	Reason    string
}

// DriverCancelScheduledCommand is used by a driver to cancel a claimed scheduled order;
// the order is re-opened (reverts to StatusScheduled) with an increased incentive bonus.
type DriverCancelScheduledCommand struct {
	OrderID  types.ID
	DriverID types.ID
}

// ListScheduledByPassengerCommand specifies the filter for listing a passenger's scheduled orders.
type ListScheduledByPassengerCommand struct {
	PassengerID types.ID
}

// CreateScheduled validates and persists a new scheduled order.
//
// Business rules:
//   - scheduled_at must be at least 30 minutes in the future.
//   - cancel_deadline_at is computed as scheduled_at minus schedule_window_mins.
//   - A passenger may not have another active (including scheduled) order at creation time.
func (s *Service) CreateScheduled(ctx context.Context, cmd CreateScheduledCommand) (types.ID, error) {
	if cmd.PassengerID == "" || cmd.RideType == "" {
		return "", ErrBadRequest
	}
	if cmd.ScheduleWindowMins <= 0 {
		return "", ErrBadRequest
	}
	now := time.Now()
	if cmd.ScheduledAt.Before(now.Add(30 * time.Minute)) {
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
	est := types.Money{Amount: 0, Currency: "TWD"}
	if s.pricing != nil {
		if m, err := s.pricing.Estimate(ctx, distanceKm(cmd.Pickup, cmd.Dropoff), cmd.RideType); err == nil {
			est = m
		}
	}

	cancelDeadlineAt := cmd.ScheduledAt.Add(-time.Duration(cmd.ScheduleWindowMins) * time.Minute)
	windowMins := cmd.ScheduleWindowMins

	o := &Order{
		ID:                 id,
		PassengerID:        cmd.PassengerID,
		Status:             StatusScheduled,
		StatusVersion:      0,
		Pickup:             cmd.Pickup,
		Dropoff:            cmd.Dropoff,
		RideType:           cmd.RideType,
		EstimatedFee:       est,
		OrderType:          "scheduled",
		ScheduledAt:        &cmd.ScheduledAt,
		ScheduleWindowMins: &windowMins,
		CancelDeadlineAt:   &cancelDeadlineAt,
		IncentiveBonus:     0,
		CreatedAt:          now,
	}
	if err := s.store.CreateScheduled(ctx, o); err != nil {
		return "", err
	}
	_ = s.store.AppendEvent(ctx, &Event{
		OrderID:    id,
		FromStatus: StatusNone,
		ToStatus:   StatusScheduled,
		ActorType:  "passenger",
		ActorID:    &cmd.PassengerID,
		CreatedAt:  now,
	})
	return id, nil
}

// ListScheduledByPassenger returns all scheduled orders for a given passenger.
func (s *Service) ListScheduledByPassenger(ctx context.Context, passengerID types.ID) ([]*Order, error) {
	if passengerID == "" {
		return nil, ErrBadRequest
	}
	return s.store.ListScheduledByPassenger(ctx, passengerID)
}

// ListAvailableScheduled returns all open scheduled orders within the given time window,
// suitable for drivers browsing available work.
func (s *Service) ListAvailableScheduled(ctx context.Context, from, to time.Time) ([]*Order, error) {
	return s.store.ListAvailableScheduled(ctx, from, to)
}

// ClaimScheduled allows a driver to claim a scheduled order (StatusScheduled → StatusAssigned).
// An optimistic-lock ensures only one driver succeeds concurrently.
func (s *Service) ClaimScheduled(ctx context.Context, cmd ClaimScheduledCommand) error {
	if cmd.OrderID == "" || cmd.DriverID == "" {
		return ErrBadRequest
	}
	o, err := s.store.Get(ctx, cmd.OrderID)
	if err != nil {
		return err
	}
	if o.Status != StatusScheduled {
		return ErrInvalidState
	}
	ok, err := s.store.ClaimScheduled(ctx, cmd.OrderID, cmd.DriverID, o.StatusVersion)
	if err != nil {
		return err
	}
	if !ok {
		return ErrConflict
	}
	now := time.Now()
	_ = s.store.AppendEvent(ctx, &Event{
		OrderID:    cmd.OrderID,
		FromStatus: StatusScheduled,
		ToStatus:   StatusAssigned,
		ActorType:  "driver",
		ActorID:    &cmd.DriverID,
		CreatedAt:  now,
	})
	return nil
}

// CancelScheduledByPassenger cancels a scheduled (or assigned) order on behalf of the passenger.
// If the current time is past cancel_deadline_at, the cancellation is still accepted for MVP
// (fee enforcement can be added later).
func (s *Service) CancelScheduledByPassenger(ctx context.Context, cmd CancelScheduledCommand) error {
	if cmd.OrderID == "" {
		return ErrBadRequest
	}
	return s.applyTransition(ctx, cmd.OrderID, transitionParams{
		to:        StatusCancelled,
		actorType: "passenger",
	})
}

// CancelScheduledByDriver re-opens a claimed scheduled order (StatusAssigned → StatusScheduled),
// clears the driver assignment, and increases the incentive bonus to attract a new driver.
func (s *Service) CancelScheduledByDriver(ctx context.Context, cmd DriverCancelScheduledCommand) error {
	if cmd.OrderID == "" || cmd.DriverID == "" {
		return ErrBadRequest
	}
	o, err := s.store.Get(ctx, cmd.OrderID)
	if err != nil {
		return err
	}
	if o.Status != StatusAssigned {
		return ErrInvalidState
	}
	ok, err := s.store.ReopenScheduled(ctx, cmd.OrderID, o.StatusVersion, driverCancelBonusIncrement)
	if err != nil {
		return err
	}
	if !ok {
		return ErrConflict
	}
	now := time.Now()
	_ = s.store.AppendEvent(ctx, &Event{
		OrderID:    cmd.OrderID,
		FromStatus: StatusAssigned,
		ToStatus:   StatusScheduled,
		ActorType:  "driver",
		ActorID:    &cmd.DriverID,
		CreatedAt:  now,
	})
	return nil
}

// RunScheduleIncentiveTicker periodically increases the incentive_bonus for scheduled orders
// that are within the schedule_window_mins window but have not yet been claimed.
func (s *Service) RunScheduleIncentiveTicker(ctx context.Context) {
	ticker := time.NewTicker(incentiveTickerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.store.BumpIncentiveBonusForApproaching(ctx, incentiveTickerBump)
		}
	}
}

// RunScheduleExpireTicker periodically expires scheduled orders whose scheduled_at time
// has passed the end of their schedule_window_mins without being claimed.
func (s *Service) RunScheduleExpireTicker(ctx context.Context) {
	ticker := time.NewTicker(expireTickerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.store.ExpireOverdueScheduled(ctx)
		}
	}
}

