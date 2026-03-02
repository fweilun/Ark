// README: Calendar service — creates/edits/deletes events and manages order-event links.
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
	ListEventsByUser(ctx context.Context, uid types.ID) ([]*Event, error)
	CreateSchedule(ctx context.Context, sc *Schedule) error
	GetSchedule(ctx context.Context, uid, eventID types.ID) (*Schedule, error)
	ListSchedulesByUser(ctx context.Context, uid types.ID) ([]*Schedule, error)
	CreateOrderEvent(ctx context.Context, oe *OrderEvent) error
	GetOrderEvent(ctx context.Context, id types.ID) (*OrderEvent, error)
	DeleteOrderEvent(ctx context.Context, id types.ID) error
	ListOrderEventsByUser(ctx context.Context, uid types.ID) ([]*OrderEvent, error)
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
	ErrForbidden  = errors.New("calendar: forbidden")
)

// CreateEventCommand holds the fields required to create a new calendar event.
type CreateEventCommand struct {
	UID         types.ID
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

// CreateOrderEventCommand creates a ride order and links it to an existing calendar event.
type CreateOrderEventCommand struct {
	UID         types.ID // user who owns the order-event link
	EventID     types.ID // existing event to link the order to
	PassengerID types.ID
	Pickup      types.Point
	Dropoff     types.Point
	RideType    string
}

// CancelOrderEventCommand cancels the ride order and removes the order-event link.
type CancelOrderEventCommand struct {
	UID          types.ID
	OrderEventID types.ID
}

// CreateEvent persists a new calendar event, registers the user as an attendee, and returns the event ID.
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
	if cmd.UID != "" {
		_ = s.store.CreateSchedule(ctx, &Schedule{UID: cmd.UID, EventID: e.ID})
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

// CreateOrderEvent creates a ride order and a link between the order and the calendar event.
// If the order-event insert fails, the order is cancelled as a best-effort cleanup.
func (s *Service) CreateOrderEvent(ctx context.Context, cmd CreateOrderEventCommand) (*OrderEvent, error) {
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
	oe := &OrderEvent{
		ID:        newID(),
		EventID:   cmd.EventID,
		OrderID:   orderID,
		UID:       cmd.UID,
		CreatedAt: time.Now(),
	}
	if err := s.store.CreateOrderEvent(ctx, oe); err != nil {
		// Best-effort: cancel the order to avoid an orphaned ride request.
		_ = s.order.Cancel(ctx, order.CancelCommand{
			OrderID:   orderID,
			ActorType: "system",
			Reason:    "order_event_creation_failed",
		})
		return nil, err
	}
	return oe, nil
}

// CancelOrderEvent cancels the linked ride order and removes the order-event link.
// Only the owner of the order-event link may cancel it.
func (s *Service) CancelOrderEvent(ctx context.Context, cmd CancelOrderEventCommand) error {
	if cmd.UID == "" || cmd.OrderEventID == "" {
		return ErrBadRequest
	}
	oe, err := s.store.GetOrderEvent(ctx, cmd.OrderEventID)
	if err != nil {
		return err
	}
	if oe.UID != cmd.UID {
		return ErrForbidden
	}
	if err := s.order.Cancel(ctx, order.CancelCommand{
		OrderID:   oe.OrderID,
		ActorType: "passenger",
		Reason:    "removed_from_calendar",
	}); err != nil {
		return err
	}
	return s.store.DeleteOrderEvent(ctx, cmd.OrderEventID)
}

// ListAllEvents returns all calendar events for the given user.
func (s *Service) ListAllEvents(ctx context.Context, uid types.ID) ([]*Event, error) {
	if uid == "" {
		return nil, ErrBadRequest
	}
	return s.store.ListEventsByUser(ctx, uid)
}

// ListAllOrders returns all order-event links for the given user.
func (s *Service) ListAllOrders(ctx context.Context, uid types.ID) ([]*OrderEvent, error) {
	if uid == "" {
		return nil, ErrBadRequest
	}
	return s.store.ListOrderEventsByUser(ctx, uid)
}

func newID() types.ID {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return types.ID(hex.EncodeToString(b[:]))
}
