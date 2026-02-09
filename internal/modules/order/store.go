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
            ride_type, estimated_fee, actual_fee, created_at
        ) VALUES (
            $1, $2, $3, $4, $5,
            $6, $7, $8, $9,
            $10, $11, $12, $13
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
        o.CreatedAt,
    )
    return err
}

func (s *Store) Get(ctx context.Context, id types.ID) (*Order, error) {
    row := s.db.QueryRow(ctx, `
        SELECT id, passenger_id, driver_id, status, status_version,
               pickup_lat, pickup_lng, dropoff_lat, dropoff_lng,
               ride_type, estimated_fee, actual_fee,
               created_at, matched_at, accepted_at, started_at, completed_at, cancelled_at, cancellation_reason
        FROM orders
        WHERE id = $1`, string(id),
    )

    var o Order
    var driverID sql.NullString
    var actualFee sql.NullInt64
    var matchedAt, acceptedAt, startedAt, completedAt, cancelledAt sql.NullTime
    var cancelReason sql.NullString

    err := row.Scan(
        &o.ID, &o.PassengerID, &driverID, &o.Status, &o.StatusVersion,
        &o.Pickup.Lat, &o.Pickup.Lng, &o.Dropoff.Lat, &o.Dropoff.Lng,
        &o.RideType, &o.EstimatedFee.Amount, &actualFee,
        &o.CreatedAt, &matchedAt, &acceptedAt, &startedAt, &completedAt, &cancelledAt, &cancelReason,
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
            matched_at = CASE WHEN $1 = 'driver_found' THEN NOW() ELSE matched_at END,
            accepted_at = CASE WHEN $1 = 'ride_accepted' THEN NOW() ELSE accepted_at END,
            started_at = CASE WHEN $1 = 'trip_started' THEN NOW() ELSE started_at END,
            completed_at = CASE WHEN $1 = 'trip_complete' THEN NOW() ELSE completed_at END,
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
    row := s.db.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM orders
            WHERE passenger_id = $1
              AND status IN ('created','matched','accepted','in_progress')
        )`, string(passengerID),
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
