package planner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("plan not found")
var ErrBadRequest = errors.New("bad request")

// CreatePlanCommand contains the data needed to create a new plan.
type CreatePlanCommand struct {
	PassengerID string
	Origin      string
	Destination string
	PickupAt    time.Time
	Notes       string
}

// UpdatePlanCommand contains fields that may be modified on an existing plan.
type UpdatePlanCommand struct {
	ID          string
	Origin      string
	Destination string
	PickupAt    time.Time
	Status      Status
	Notes       string
}

// Service defines the public business operations for the planner module.
// It is consumed by the aiusage module via dependency injection.
type Service interface {
	CreatePlan(ctx context.Context, cmd CreatePlanCommand) (*Plan, error)
	GetPlan(ctx context.Context, id string) (*Plan, error)
	UpdatePlan(ctx context.Context, cmd UpdatePlanCommand) (*Plan, error)
	DeletePlan(ctx context.Context, id string) error
	ListPlans(ctx context.Context, passengerID string) ([]*Plan, error)
}

// plannerService is the private implementation of Service.
type plannerService struct {
	store *Store
}

// NewService constructs the plannerService with the given Store.
func NewService(store *Store) Service {
	return &plannerService{store: store}
}

func (s *plannerService) CreatePlan(ctx context.Context, cmd CreatePlanCommand) (*Plan, error) {
	if cmd.PassengerID == "" || cmd.Destination == "" {
		return nil, ErrBadRequest
	}
	now := time.Now().UTC()
	p := &Plan{
		ID:          newPlanID(),
		PassengerID: cmd.PassengerID,
		Origin:      cmd.Origin,
		Destination: cmd.Destination,
		PickupAt:    cmd.PickupAt.UTC(),
		Status:      StatusDraft,
		Notes:       cmd.Notes,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return s.store.Create(ctx, p)
}

func (s *plannerService) GetPlan(ctx context.Context, id string) (*Plan, error) {
	if id == "" {
		return nil, ErrBadRequest
	}
	p, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("planner: get: %w", err)
	}
	return p, nil
}

func (s *plannerService) UpdatePlan(ctx context.Context, cmd UpdatePlanCommand) (*Plan, error) {
	if cmd.ID == "" {
		return nil, ErrBadRequest
	}
	existing, err := s.store.Get(ctx, cmd.ID)
	if err != nil {
		return nil, fmt.Errorf("planner: update get: %w", err)
	}
	// Apply only non-zero fields.
	if cmd.Origin != "" {
		existing.Origin = cmd.Origin
	}
	if cmd.Destination != "" {
		existing.Destination = cmd.Destination
	}
	if !cmd.PickupAt.IsZero() {
		existing.PickupAt = cmd.PickupAt.UTC()
	}
	if cmd.Status != "" {
		existing.Status = cmd.Status
	}
	if cmd.Notes != "" {
		existing.Notes = cmd.Notes
	}
	return s.store.Update(ctx, existing)
}

func (s *plannerService) DeletePlan(ctx context.Context, id string) error {
	if id == "" {
		return ErrBadRequest
	}
	return s.store.Delete(ctx, id)
}

func (s *plannerService) ListPlans(ctx context.Context, passengerID string) ([]*Plan, error) {
	if passengerID == "" {
		return nil, ErrBadRequest
	}
	return s.store.ListByPassenger(ctx, passengerID)
}

func newPlanID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
