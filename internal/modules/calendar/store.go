// README: Calendar store backed by PostgreSQL.
package calendar

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"ark/internal/types"
)

// Store handles persistence for calendar events and schedules.
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
	if errors.Is(err, sql.ErrNoRows) {
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

// CreateSchedule inserts a new schedule entry linking a user to an event.
func (s *Store) CreateSchedule(ctx context.Context, sc *Schedule) error {
	var orderID *string
	if sc.TiedOrder != nil {
		v := string(*sc.TiedOrder)
		orderID = &v
	}
	_, err := s.db.Exec(ctx, `
        INSERT INTO calendar_schedules (uid, event_id, tied_order)
        VALUES ($1, $2, $3)`,
		string(sc.UID), string(sc.EventID), orderID,
	)
	return err
}

// GetSchedule retrieves a schedule entry by user ID and event ID.
func (s *Store) GetSchedule(ctx context.Context, uid, eventID types.ID) (*Schedule, error) {
	row := s.db.QueryRow(ctx, `
        SELECT uid, event_id, tied_order
        FROM calendar_schedules
        WHERE uid = $1 AND event_id = $2`,
		string(uid), string(eventID),
	)
	var sc Schedule
	var tiedOrder sql.NullString
	err := row.Scan(&sc.UID, &sc.EventID, &tiedOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if tiedOrder.Valid {
		id := types.ID(tiedOrder.String)
		sc.TiedOrder = &id
	}
	return &sc, nil
}

// UpdateScheduleTiedOrder sets or clears the tied_order on a schedule entry.
func (s *Store) UpdateScheduleTiedOrder(ctx context.Context, uid, eventID types.ID, orderID *types.ID) error {
	var o *string
	if orderID != nil {
		v := string(*orderID)
		o = &v
	}
	tag, err := s.db.Exec(ctx, `
        UPDATE calendar_schedules
        SET tied_order = $1
        WHERE uid = $2 AND event_id = $3`,
		o, string(uid), string(eventID),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListSchedulesByUser returns all schedule entries for a given user.
func (s *Store) ListSchedulesByUser(ctx context.Context, uid types.ID) ([]*Schedule, error) {
	rows, err := s.db.Query(ctx, `
        SELECT uid, event_id, tied_order
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
		var tiedOrder sql.NullString
		if err := rows.Scan(&sc.UID, &sc.EventID, &tiedOrder); err != nil {
			return nil, err
		}
		if tiedOrder.Valid {
			id := types.ID(tiedOrder.String)
			sc.TiedOrder = &id
		}
		schedules = append(schedules, &sc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return schedules, nil
}
