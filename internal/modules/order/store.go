// README: Order store backed by PostgreSQL (minimal methods for MVP).
package order

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ark/internal/types"
)

// activeStatuses is the set of order statuses that block a passenger from creating a new order.
var activeStatuses = []Status{
	StatusScheduled,
	StatusWaiting,
	StatusAssigned,
	StatusApproaching,
	StatusArrived,
	StatusDriving,
	StatusPayment,
}

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, o *Order) error {
	_, err := s.db.Exec(ctx, `
        INSERT INTO orders (
            id, passenger_id, driver_id, status, status_version,
            pickup_lat, pickup_lng, dropoff_lat, dropoff_lng,
            ride_type, estimated_fee, actual_fee, order_type, created_at
        ) VALUES (
            $1, $2, $3, $4, $5,
            $6, $7, $8, $9,
            $10, $11, $12, $13, $14
        )`,
		string(o.ID),
		string(o.PassengerID),
		toStringPtr(o.DriverID),
		string(o.Status),
		o.StatusVersion,
		o.Pickup.Lat, o.Pickup.Lng,
		o.Dropoff.Lat, o.Dropoff.Lng,
		o.RideType,
		o.EstimatedFee.Amount,
		toIntPtr(o.ActualFee),
		o.OrderType,
		o.CreatedAt,
	)
	return err
}

func (s *Store) Get(ctx context.Context, id types.ID) (*Order, error) {
	row := s.db.QueryRow(ctx, `
        SELECT id, passenger_id, driver_id, status, status_version,
               pickup_lat, pickup_lng, dropoff_lat, dropoff_lng,
               ride_type, estimated_fee, actual_fee,
               created_at, matched_at, accepted_at, started_at, completed_at, cancelled_at, cancellation_reason,
               order_type, scheduled_at, schedule_window_mins, cancel_deadline_at, incentive_bonus, assigned_at
        FROM orders
        WHERE id = $1`, string(id),
	)

	var o Order
	var driverID sql.NullString
	var actualFee sql.NullInt64
	var matchedAt, acceptedAt, startedAt, completedAt, cancelledAt sql.NullTime
	var cancelReason sql.NullString
	var orderType sql.NullString
	var scheduledAt, cancelDeadlineAt, assignedAt sql.NullTime
	var scheduleWindowMins sql.NullInt32
	var incentiveBonus sql.NullInt64

	err := row.Scan(
		&o.ID, &o.PassengerID, &driverID, &o.Status, &o.StatusVersion,
		&o.Pickup.Lat, &o.Pickup.Lng, &o.Dropoff.Lat, &o.Dropoff.Lng,
		&o.RideType, &o.EstimatedFee.Amount, &actualFee,
		&o.CreatedAt, &matchedAt, &acceptedAt, &startedAt, &completedAt, &cancelledAt, &cancelReason,
		&orderType, &scheduledAt, &scheduleWindowMins, &cancelDeadlineAt, &incentiveBonus, &assignedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if driverID.Valid {
		d := types.ID(driverID.String)
		o.DriverID = &d
	}
	if actualFee.Valid {
		v := types.Money{Amount: actualFee.Int64, Currency: o.EstimatedFee.Currency}
		o.ActualFee = &v
	}
	o.MatchedAt = toTimePtr(matchedAt)
	o.AcceptedAt = toTimePtr(acceptedAt)
	o.StartedAt = toTimePtr(startedAt)
	o.CompletedAt = toTimePtr(completedAt)
	o.CancelledAt = toTimePtr(cancelledAt)
	if cancelReason.Valid {
		o.CancelReason = &cancelReason.String
	}
	if o.EstimatedFee.Currency == "" {
		o.EstimatedFee.Currency = "TWD"
	}
	if orderType.Valid {
		o.OrderType = orderType.String
	}
	o.ScheduledAt = toTimePtr(scheduledAt)
	o.CancelDeadlineAt = toTimePtr(cancelDeadlineAt)
	o.AssignedAt = toTimePtr(assignedAt)
	if scheduleWindowMins.Valid {
		v := int(scheduleWindowMins.Int32)
		o.ScheduleWindowMins = &v
	}
	if incentiveBonus.Valid {
		o.IncentiveBonus = incentiveBonus.Int64
	}
	return &o, nil
}

func (s *Store) UpdateStatus(ctx context.Context, id types.ID, from, to Status, version int, driverID *types.ID) (bool, error) {
	var d *string
	if driverID != nil {
		v := string(*driverID)
		d = &v
	}
	tag, err := s.db.Exec(ctx, `
        UPDATE orders
        SET status = $1,
            status_version = status_version + 1,
            driver_id = COALESCE($2, driver_id),
            matched_at = CASE WHEN $1 = 'approaching' THEN NOW() ELSE matched_at END,
            accepted_at = CASE WHEN $1 = 'approaching' THEN NOW() ELSE accepted_at END,
            started_at = CASE WHEN $1 = 'driving' THEN NOW() ELSE started_at END,
            completed_at = CASE WHEN $1 IN ('payment','complete') THEN NOW() ELSE completed_at END,
            cancelled_at = CASE WHEN $1 = 'cancelled' THEN NOW() ELSE cancelled_at END
        WHERE id = $3 AND status = $4 AND status_version = $5`,
		string(to),
		d,
		string(id),
		string(from),
		version,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) AppendEvent(ctx context.Context, e *Event) error {
	_, err := s.db.Exec(ctx, `
        INSERT INTO order_state_events (
            order_id, from_status, to_status, actor_type, actor_id, created_at
        ) VALUES ($1, $2, $3, $4, $5, $6)`,
		string(e.OrderID),
		string(e.FromStatus),
		string(e.ToStatus),
		e.ActorType,
		toStringPtr(e.ActorID),
		e.CreatedAt,
	)
	return err
}

func (s *Store) HasActiveByPassenger(ctx context.Context, passengerID types.ID) (bool, error) {
	statuses := make([]string, len(activeStatuses))
	for i, st := range activeStatuses {
		statuses[i] = string(st)
	}
	row := s.db.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM orders
            WHERE passenger_id = $1
              AND status = ANY($2)
        )`, string(passengerID), statuses,
    )
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func toStringPtr(v *types.ID) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}

func toIntPtr(v *types.Money) *int64 {
	if v == nil {
		return nil
	}
	n := v.Amount
	return &n
}

func toTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

// CreateScheduled inserts a scheduled order with all scheduled-specific fields.
func (s *Store) CreateScheduled(ctx context.Context, o *Order) error {
	_, err := s.db.Exec(ctx, `
        INSERT INTO orders (
            id, passenger_id, status, status_version,
            pickup_lat, pickup_lng, dropoff_lat, dropoff_lng,
            ride_type, estimated_fee, order_type,
            scheduled_at, schedule_window_mins, cancel_deadline_at, incentive_bonus,
            created_at
        ) VALUES (
            $1, $2, $3, $4,
            $5, $6, $7, $8,
            $9, $10, $11,
            $12, $13, $14, $15,
            $16
        )`,
		string(o.ID),
		string(o.PassengerID),
		string(o.Status),
		o.StatusVersion,
		o.Pickup.Lat, o.Pickup.Lng,
		o.Dropoff.Lat, o.Dropoff.Lng,
		o.RideType,
		o.EstimatedFee.Amount,
		o.OrderType,
		o.ScheduledAt,
		o.ScheduleWindowMins,
		o.CancelDeadlineAt,
		o.IncentiveBonus,
		o.CreatedAt,
	)
	return err
}

// ListScheduledByPassenger returns all scheduled-type orders for a passenger, newest first.
func (s *Store) ListScheduledByPassenger(ctx context.Context, passengerID types.ID) ([]*Order, error) {
	rows, err := s.db.Query(ctx, `
        SELECT id, passenger_id, driver_id, status, status_version,
               pickup_lat, pickup_lng, dropoff_lat, dropoff_lng,
               ride_type, estimated_fee,
               created_at, scheduled_at, cancel_deadline_at, incentive_bonus, assigned_at,
               order_type, schedule_window_mins
        FROM orders
        WHERE passenger_id = $1 AND order_type = 'scheduled'
        ORDER BY created_at DESC`, string(passengerID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrderRows(rows)
}

// ListAvailableScheduled returns open (status='scheduled') orders within the given time window.
func (s *Store) ListAvailableScheduled(ctx context.Context, from, to time.Time) ([]*Order, error) {
	rows, err := s.db.Query(ctx, `
        SELECT id, passenger_id, driver_id, status, status_version,
               pickup_lat, pickup_lng, dropoff_lat, dropoff_lng,
               ride_type, estimated_fee,
               created_at, scheduled_at, cancel_deadline_at, incentive_bonus, assigned_at,
               order_type, schedule_window_mins
        FROM orders
        WHERE status = 'scheduled' AND scheduled_at BETWEEN $1 AND $2
        ORDER BY scheduled_at ASC`, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrderRows(rows)
}

// ClaimScheduled atomically moves a scheduled order from 'scheduled' to 'assigned' for a driver.
// Returns (false, nil) if the optimistic-lock check failed (another driver got there first).
func (s *Store) ClaimScheduled(ctx context.Context, orderID, driverID types.ID, expectVersion int) (bool, error) {
	tag, err := s.db.Exec(ctx, `
        UPDATE orders
        SET status = 'assigned',
            driver_id = $1,
            assigned_at = NOW(),
            status_version = status_version + 1
        WHERE id = $2 AND status = 'scheduled' AND status_version = $3`,
		string(driverID),
		string(orderID),
		expectVersion,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// ReopenScheduled moves an 'assigned' order back to 'scheduled' (driver cancel),
// clears the driver assignment, and adds bonus to incentive_bonus.
// Returns (false, nil) if the optimistic-lock check failed.
func (s *Store) ReopenScheduled(ctx context.Context, orderID types.ID, expectVersion int, bonus int64) (bool, error) {
	tag, err := s.db.Exec(ctx, `
        UPDATE orders
        SET status = 'scheduled',
            driver_id = NULL,
            assigned_at = NULL,
            incentive_bonus = incentive_bonus + $1,
            status_version = status_version + 1
        WHERE id = $2 AND status = 'assigned' AND status_version = $3`,
		bonus,
		string(orderID),
		expectVersion,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// BumpIncentiveBonusForApproaching increases incentive_bonus for scheduled orders
// whose scheduled_at is within the next schedule_window_mins minutes and are still unclaimed.
func (s *Store) BumpIncentiveBonusForApproaching(ctx context.Context, bump int64) error {
	_, err := s.db.Exec(ctx, `
        UPDATE orders
        SET incentive_bonus = incentive_bonus + $1
        WHERE status = 'scheduled'
          AND scheduled_at <= NOW() + (schedule_window_mins * INTERVAL '1 minute')
          AND scheduled_at > NOW()`,
		bump,
	)
	return err
}

// ExpireOverdueScheduled marks scheduled orders as 'expired' when scheduled_at has passed
// the end of their schedule_window_mins without being claimed.
func (s *Store) ExpireOverdueScheduled(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
        UPDATE orders
        SET status = 'expired',
            status_version = status_version + 1
        WHERE status = 'scheduled'
          AND scheduled_at + (schedule_window_mins * INTERVAL '1 minute') < NOW()`,
	)
	return err
}

// scanOrderRows is a helper that scans the subset of columns returned by ListScheduledByPassenger
// and ListAvailableScheduled.
func scanOrderRows(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]*Order, error) {
	var orders []*Order
	for rows.Next() {
		var o Order
		var driverID sql.NullString
		var scheduledAt, cancelDeadlineAt, assignedAt sql.NullTime
		var scheduleWindowMins sql.NullInt32
		var incentiveBonus sql.NullInt64
		var orderType sql.NullString

		err := rows.Scan(
			&o.ID, &o.PassengerID, &driverID, &o.Status, &o.StatusVersion,
			&o.Pickup.Lat, &o.Pickup.Lng, &o.Dropoff.Lat, &o.Dropoff.Lng,
			&o.RideType, &o.EstimatedFee.Amount,
			&o.CreatedAt, &scheduledAt, &cancelDeadlineAt, &incentiveBonus, &assignedAt,
			&orderType, &scheduleWindowMins,
		)
		if err != nil {
			return nil, err
		}
		if driverID.Valid {
			d := types.ID(driverID.String)
			o.DriverID = &d
		}
		o.ScheduledAt = toTimePtr(scheduledAt)
		o.CancelDeadlineAt = toTimePtr(cancelDeadlineAt)
		o.AssignedAt = toTimePtr(assignedAt)
		if scheduleWindowMins.Valid {
			v := int(scheduleWindowMins.Int32)
			o.ScheduleWindowMins = &v
		}
		if incentiveBonus.Valid {
			o.IncentiveBonus = incentiveBonus.Int64
		}
		if orderType.Valid {
			o.OrderType = orderType.String
		}
		if o.EstimatedFee.Currency == "" {
			o.EstimatedFee.Currency = "TWD"
		}
		orders = append(orders, &o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}
