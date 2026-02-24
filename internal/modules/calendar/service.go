// README: Calendar service — creates/edits/deletes events and manages schedule-order ties.
package calendar

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"ark/internal/types"
)

// Service orchestrates calendar event and schedule logic.
type Service struct {
	store *Store
}

// NewService creates a Service backed by the given Store.
func NewService(store *Store) *Service {
	return &Service{store: store}
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

// CreateAndTieOrderCommand creates a new calendar event and ties it to a ride order
// via a Schedule entry for the given user.
type CreateAndTieOrderCommand struct {
	UID         types.ID
	From        time.Time
	To          time.Time
	Title       string
	Description string
	OrderID     types.ID
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

// CreateAndTieOrder creates a new calendar event and a Schedule entry
// that links the given user to the event and ties the event to an order.
func (s *Service) CreateAndTieOrder(ctx context.Context, cmd CreateAndTieOrderCommand) (*Schedule, error) {
	if cmd.UID == "" || cmd.OrderID == "" || cmd.Title == "" {
		return nil, ErrBadRequest
	}
	if !cmd.From.Before(cmd.To) {
		return nil, ErrBadRequest
	}
	e := &Event{
		ID:          newID(),
		From:        cmd.From,
		To:          cmd.To,
		Title:       cmd.Title,
		Description: cmd.Description,
	}
	if err := s.store.CreateEvent(ctx, e); err != nil {
		return nil, err
	}
	sc := &Schedule{
		UID:       cmd.UID,
		EventID:   e.ID,
		TiedOrder: &cmd.OrderID,
	}
	if err := s.store.CreateSchedule(ctx, sc); err != nil {
		return nil, err
	}
	return sc, nil
}

// UntieOrder clears the tied_order from a user's schedule entry for the given event.
func (s *Service) UntieOrder(ctx context.Context, cmd UntieOrderCommand) error {
	if cmd.UID == "" || cmd.EventID == "" {
		return ErrBadRequest
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
