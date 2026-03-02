// README: Calendar store backed by PostgreSQL.
package calendar

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ark/internal/types"
)

// Store handles persistence for calendar events, schedules, and order-events.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a Store backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// CreateEvent inserts a new calendar event.
func (s *Store) CreateEvent(ctx context.Context, e *Event) error {
	_, err := s.db.Exec(ctx, `
        INSERT INTO calendar_events (id, "from", "to", title, description)
        VALUES ($1, $2, $3, $4, $5)`,
		string(e.ID), e.From, e.To, e.Title, e.Description,
	)
	return err
}

// GetEvent retrieves a calendar event by ID.
func (s *Store) GetEvent(ctx context.Context, id types.ID) (*Event, error) {
	row := s.db.QueryRow(ctx, `
        SELECT id, "from", "to", title, description
        FROM calendar_events
        WHERE id = $1`, string(id),
	)
	var e Event
	err := row.Scan(&e.ID, &e.From, &e.To, &e.Title, &e.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// UpdateEvent updates the fields of an existing calendar event.
func (s *Store) UpdateEvent(ctx context.Context, e *Event) error {
	tag, err := s.db.Exec(ctx, `
        UPDATE calendar_events
        SET "from" = $1, "to" = $2, title = $3, description = $4
        WHERE id = $5`,
		e.From, e.To, e.Title, e.Description, string(e.ID),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteEvent removes a calendar event by ID.
func (s *Store) DeleteEvent(ctx context.Context, id types.ID) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM calendar_events WHERE id = $1`, string(id))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListEventsByUser returns all calendar events for which the user has a schedule entry.
func (s *Store) ListEventsByUser(ctx context.Context, uid types.ID) ([]*Event, error) {
	rows, err := s.db.Query(ctx, `
        SELECT e.id, e."from", e."to", e.title, e.description
        FROM calendar_events e
        INNER JOIN calendar_schedules sc ON sc.event_id = e.id
        WHERE sc.uid = $1`, string(uid),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.From, &e.To, &e.Title, &e.Description); err != nil {
			return nil, err
		}
		events = append(events, &e)
	}
	return events, rows.Err()
}

// CreateSchedule inserts a new schedule entry linking a user to an event.
func (s *Store) CreateSchedule(ctx context.Context, sc *Schedule) error {
	_, err := s.db.Exec(ctx, `
        INSERT INTO calendar_schedules (uid, event_id)
        VALUES ($1, $2)`,
		string(sc.UID), string(sc.EventID),
	)
	return err
}

// GetSchedule retrieves a schedule entry by user ID and event ID.
func (s *Store) GetSchedule(ctx context.Context, uid, eventID types.ID) (*Schedule, error) {
	row := s.db.QueryRow(ctx, `
        SELECT uid, event_id
        FROM calendar_schedules
        WHERE uid = $1 AND event_id = $2`,
		string(uid), string(eventID),
	)
	var sc Schedule
	err := row.Scan(&sc.UID, &sc.EventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

// ListSchedulesByUser returns all schedule entries for a given user.
func (s *Store) ListSchedulesByUser(ctx context.Context, uid types.ID) ([]*Schedule, error) {
	rows, err := s.db.Query(ctx, `
        SELECT uid, event_id
        FROM calendar_schedules
        WHERE uid = $1`, string(uid),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []*Schedule
	for rows.Next() {
		var sc Schedule
		if err := rows.Scan(&sc.UID, &sc.EventID); err != nil {
			return nil, err
		}
		schedules = append(schedules, &sc)
	}
	return schedules, rows.Err()
}

// CreateOrderEvent inserts a new order-event link.
func (s *Store) CreateOrderEvent(ctx context.Context, oe *OrderEvent) error {
	_, err := s.db.Exec(ctx, `
        INSERT INTO calendar_order_events (id, event_id, order_id, uid, created_at)
        VALUES ($1, $2, $3, $4, $5)`,
		string(oe.ID), string(oe.EventID), string(oe.OrderID), string(oe.UID), oe.CreatedAt,
	)
	return err
}

// GetOrderEvent retrieves an order-event link by ID.
func (s *Store) GetOrderEvent(ctx context.Context, id types.ID) (*OrderEvent, error) {
	row := s.db.QueryRow(ctx, `
        SELECT id, event_id, order_id, uid, created_at
        FROM calendar_order_events
        WHERE id = $1`, string(id),
	)
	var oe OrderEvent
	err := row.Scan(&oe.ID, &oe.EventID, &oe.OrderID, &oe.UID, &oe.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &oe, nil
}

// DeleteOrderEvent removes an order-event link by ID.
func (s *Store) DeleteOrderEvent(ctx context.Context, id types.ID) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM calendar_order_events WHERE id = $1`, string(id))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListOrderEventsByUser returns all order-event links for a given user.
func (s *Store) ListOrderEventsByUser(ctx context.Context, uid types.ID) ([]*OrderEvent, error) {
	rows, err := s.db.Query(ctx, `
        SELECT id, event_id, order_id, uid, created_at
        FROM calendar_order_events
        WHERE uid = $1
        ORDER BY created_at DESC`, string(uid),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orderEvents []*OrderEvent
	for rows.Next() {
		var oe OrderEvent
		if err := rows.Scan(&oe.ID, &oe.EventID, &oe.OrderID, &oe.UID, &oe.CreatedAt); err != nil {
			return nil, err
		}
		orderEvents = append(orderEvents, &oe)
	}
	return orderEvents, rows.Err()
}
