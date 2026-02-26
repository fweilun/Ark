// README: Calendar service — creates/edits/deletes events and manages schedule-order ties.
package calendar

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"ark/internal/modules/order"
	"ark/internal/types"
)

// StoreInterface defines the storage operations needed by the calendar service.
type StoreInterface interface {
	CreateEvent(ctx context.Context, e *Event) error
	GetEvent(ctx context.Context, id types.ID) (*Event, error)
	UpdateEvent(ctx context.Context, e *Event) error
	DeleteEvent(ctx context.Context, id types.ID) error
	CreateSchedule(ctx context.Context, sc *Schedule) error
	GetSchedule(ctx context.Context, uid, eventID types.ID) (*Schedule, error)
	UpdateScheduleTiedOrder(ctx context.Context, uid, eventID types.ID, orderID *types.ID) error
	ListSchedulesByUser(ctx context.Context, uid types.ID) ([]*Schedule, error)
}

// OrderService defines the order operations needed by the calendar service.
type OrderService interface {
	Create(ctx context.Context, cmd order.CreateCommand) (types.ID, error)
	Cancel(ctx context.Context, cmd order.CancelCommand) error
}

// Service orchestrates calendar event and schedule logic.
type Service struct {
	store StoreInterface
	order OrderService
}

// NewService creates a Service backed by the given Store and order service.
func NewService(store StoreInterface, orderSvc OrderService) *Service {
	return &Service{store: store, order: orderSvc}
}

var (
	ErrNotFound   = errors.New("calendar: not found")
	ErrBadRequest = errors.New("calendar: bad request")
)

// CreateEventCommand holds the fields required to create a new calendar event.
type CreateEventCommand struct {
	From        time.Time
	To          time.Time
	Title       string
	Description string
}

// EditEventCommand holds the fields required to update an existing calendar event.
type EditEventCommand struct {
	ID          types.ID
	From        time.Time
	To          time.Time
	Title       string
	Description string
}

// CreateAndTieOrderCommand creates a ride order and ties it to an existing calendar event
// via a Schedule entry for the given user. The order fields mirror order.CreateCommand.
type CreateAndTieOrderCommand struct {
	UID         types.ID // user who owns the schedule entry
	EventID     types.ID // existing event to tie the order to
	PassengerID types.ID
	Pickup      types.Point
	Dropoff     types.Point
	RideType    string
}

// UntieOrderCommand removes the order link from a user's schedule entry.
type UntieOrderCommand struct {
	UID     types.ID
	EventID types.ID
}

// CreateEvent persists a new calendar event and returns its generated ID.
func (s *Service) CreateEvent(ctx context.Context, cmd CreateEventCommand) (types.ID, error) {
	if cmd.Title == "" {
		return "", ErrBadRequest
	}
	if !cmd.From.Before(cmd.To) {
		return "", ErrBadRequest
	}
	e := &Event{
		ID:          newID(),
		From:        cmd.From,
		To:          cmd.To,
		Title:       cmd.Title,
		Description: cmd.Description,
	}
	if err := s.store.CreateEvent(ctx, e); err != nil {
		return "", err
	}
	return e.ID, nil
}

// EditEvent updates the fields of an existing calendar event.
func (s *Service) EditEvent(ctx context.Context, cmd EditEventCommand) error {
	if cmd.ID == "" || cmd.Title == "" {
		return ErrBadRequest
	}
	if !cmd.From.Before(cmd.To) {
		return ErrBadRequest
	}
	return s.store.UpdateEvent(ctx, &Event{
		ID:          cmd.ID,
		From:        cmd.From,
		To:          cmd.To,
		Title:       cmd.Title,
		Description: cmd.Description,
	})
}

// DeleteEvent removes a calendar event by ID.
func (s *Service) DeleteEvent(ctx context.Context, id types.ID) error {
	if id == "" {
		return ErrBadRequest
	}
	return s.store.DeleteEvent(ctx, id)
}

// CreateAndTieOrder creates a ride order and a Schedule entry linking the user's event
// to the new order. If the schedule insert fails, the order is cancelled as a best-effort cleanup.
func (s *Service) CreateAndTieOrder(ctx context.Context, cmd CreateAndTieOrderCommand) (*Schedule, error) {
	if cmd.UID == "" || cmd.EventID == "" || cmd.PassengerID == "" || cmd.RideType == "" {
		return nil, ErrBadRequest
	}
	orderID, err := s.order.Create(ctx, order.CreateCommand{
		PassengerID: cmd.PassengerID,
		Pickup:      cmd.Pickup,
		Dropoff:     cmd.Dropoff,
		RideType:    cmd.RideType,
	})
	if err != nil {
		return nil, err
	}
	sc := &Schedule{
		UID:       cmd.UID,
		EventID:   cmd.EventID,
		TiedOrder: &orderID,
	}
	if err := s.store.CreateSchedule(ctx, sc); err != nil {
		// Best-effort: cancel the order to avoid an orphaned ride request.
		_ = s.order.Cancel(ctx, order.CancelCommand{
			OrderID:   orderID,
			ActorType: "system",
			Reason:    "schedule_creation_failed",
		})
		return nil, err
	}
	return sc, nil
}

// UntieOrder cancels the tied ride order and removes the order link from the schedule entry.
func (s *Service) UntieOrder(ctx context.Context, cmd UntieOrderCommand) error {
	if cmd.UID == "" || cmd.EventID == "" {
		return ErrBadRequest
	}
	sc, err := s.store.GetSchedule(ctx, cmd.UID, cmd.EventID)
	if err != nil {
		return err
	}
	if sc.TiedOrder == nil {
		return ErrBadRequest
	}
	if err := s.order.Cancel(ctx, order.CancelCommand{
		OrderID:   *sc.TiedOrder,
		ActorType: "passenger",
		Reason:    "removed_from_calendar",
	}); err != nil {
		return err
	}
	return s.store.UpdateScheduleTiedOrder(ctx, cmd.UID, cmd.EventID, nil)
}

// ListSchedulesByUser returns all schedule entries for the given user.
func (s *Service) ListSchedulesByUser(ctx context.Context, uid types.ID) ([]*Schedule, error) {
	if uid == "" {
		return nil, ErrBadRequest
	}
	return s.store.ListSchedulesByUser(ctx, uid)
}

func newID() types.ID {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return types.ID(hex.EncodeToString(b[:]))
}
