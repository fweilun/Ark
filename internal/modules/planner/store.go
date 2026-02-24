package planner

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store handles persistence for the planner module.
type Store struct {
	db *pgxpool.Pool
}

// NewStore returns a Store backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Create persists a new Plan and returns the stored row.
func (s *Store) Create(ctx context.Context, p *Plan) (*Plan, error) {
	row := s.db.QueryRow(ctx, `
		INSERT INTO plans (id, passenger_id, origin, destination, pickup_at, status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, passenger_id, origin, destination, pickup_at, status, notes, created_at, updated_at
	`, p.ID, p.PassengerID, p.Origin, p.Destination, p.PickupAt, p.Status, p.Notes, p.CreatedAt, p.UpdatedAt)
	return scanPlan(row)
}

// Get retrieves a single Plan by ID.
// Returns ErrNotFound if no matching row exists.
func (s *Store) Get(ctx context.Context, id string) (*Plan, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, passenger_id, origin, destination, pickup_at, status, notes, created_at, updated_at
		FROM plans WHERE id = $1
	`, id)
	p, err := scanPlan(row)
	if err != nil {
		return nil, fmt.Errorf("planner store get: %w", err)
	}
	return p, nil
}

// Update applies mutable fields (origin, destination, pickup_at, status, notes) to an existing Plan.
func (s *Store) Update(ctx context.Context, p *Plan) (*Plan, error) {
	now := time.Now().UTC()
	row := s.db.QueryRow(ctx, `
		UPDATE plans
		SET origin = $2, destination = $3, pickup_at = $4, status = $5, notes = $6, updated_at = $7
		WHERE id = $1
		RETURNING id, passenger_id, origin, destination, pickup_at, status, notes, created_at, updated_at
	`, p.ID, p.Origin, p.Destination, p.PickupAt, p.Status, p.Notes, now)
	updated, err := scanPlan(row)
	if err != nil {
		return nil, fmt.Errorf("planner store update: %w", err)
	}
	return updated, nil
}

// Delete removes a Plan by ID. It is idempotent.
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM plans WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("planner store delete: %w", err)
	}
	return nil
}

// ListByPassenger returns all Plans for a given passenger, ordered by pickup_at ascending.
func (s *Store) ListByPassenger(ctx context.Context, passengerID string) ([]*Plan, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, passenger_id, origin, destination, pickup_at, status, notes, created_at, updated_at
		FROM plans WHERE passenger_id = $1 ORDER BY pickup_at ASC
	`, passengerID)
	if err != nil {
		return nil, fmt.Errorf("planner store list: %w", err)
	}
	defer rows.Close()

	var plans []*Plan
	for rows.Next() {
		p := &Plan{}
		if err := rows.Scan(&p.ID, &p.PassengerID, &p.Origin, &p.Destination,
			&p.PickupAt, &p.Status, &p.Notes, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("planner store list scan: %w", err)
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

// scanPlan is a helper that reads a single Plan from a pgx.Row.
type scannable interface {
	Scan(dest ...any) error
}

func scanPlan(row scannable) (*Plan, error) {
	p := &Plan{}
	err := row.Scan(&p.ID, &p.PassengerID, &p.Origin, &p.Destination,
		&p.PickupAt, &p.Status, &p.Notes, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}
