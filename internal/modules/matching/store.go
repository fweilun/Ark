// README: Matching store backed by Redis GEO and Postgres for order notification tracking.
package matching

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"ark/internal/modules/order"
	"ark/internal/types"
)

type Store struct {
	redis *redis.Client
	db    *pgxpool.Pool
}

func NewStore(redis *redis.Client, db *pgxpool.Pool) *Store {
	return &Store{redis: redis, db: db}
}

func (s *Store) AddCandidate(ctx context.Context, c Candidate) error {
	return errors.New("not implemented")
}

func (s *Store) RemoveCandidate(ctx context.Context, id types.ID) error {
	return errors.New("not implemented")
}

func (s *Store) NearbyDrivers(ctx context.Context, p types.Point, radiusKm float64) ([]types.ID, error) {
	return nil, errors.New("not implemented")
}

// GetMostUrgentNotifiable returns the most urgent order with status 'scheduled' or
// 'waiting' that is not currently in a notification cooldown period, along with its
// existing notification record (nil if never notified).
// Returns (nil, nil, nil) when no eligible order exists.
func (s *Store) GetMostUrgentNotifiable(ctx context.Context) (*order.Order, *OrderNotification, error) {
	row := s.db.QueryRow(ctx, `
        SELECT o.id, o.passenger_id, o.status, o.status_version,
               o.pickup_lat, o.pickup_lng, o.dropoff_lat, o.dropoff_lng,
               o.ride_type, o.estimated_fee, o.created_at,
               o.order_type, o.scheduled_at,
               onotif.notify_count, onotif.last_notified_at, onotif.next_notifiable_at
        FROM orders o
        LEFT JOIN order_notifications onotif ON onotif.order_id = o.id
        WHERE o.status IN ('scheduled', 'waiting')
          AND (onotif.order_id IS NULL OR onotif.next_notifiable_at <= NOW())
        ORDER BY COALESCE(o.scheduled_at, o.created_at) ASC
        LIMIT 1`)

	var (
		o                order.Order
		orderType        *string
		scheduledAt      *time.Time
		notifyCount      *int32
		lastNotifiedAt   *time.Time
		nextNotifiableAt *time.Time
	)

	err := row.Scan(
		&o.ID, &o.PassengerID, &o.Status, &o.StatusVersion,
		&o.Pickup.Lat, &o.Pickup.Lng, &o.Dropoff.Lat, &o.Dropoff.Lng,
		&o.RideType, &o.EstimatedFee.Amount, &o.CreatedAt,
		&orderType, &scheduledAt,
		&notifyCount, &lastNotifiedAt, &nextNotifiableAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	if orderType != nil {
		o.OrderType = *orderType
	}
	o.ScheduledAt = scheduledAt
	if o.EstimatedFee.Currency == "" {
		o.EstimatedFee.Currency = "TWD"
	}

	var on *OrderNotification
	if notifyCount != nil && lastNotifiedAt != nil && nextNotifiableAt != nil {
		on = &OrderNotification{
			OrderID:          o.ID,
			NotifyCount:      int(*notifyCount),
			LastNotifiedAt:   *lastNotifiedAt,
			NextNotifiableAt: *nextNotifiableAt,
		}
	}

	return &o, on, nil
}

// UpsertOrderNotification inserts or updates the notification tracking record for an order.
func (s *Store) UpsertOrderNotification(ctx context.Context, on *OrderNotification) error {
	_, err := s.db.Exec(ctx, `
        INSERT INTO order_notifications (order_id, notify_count, last_notified_at, next_notifiable_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (order_id) DO UPDATE
            SET notify_count       = EXCLUDED.notify_count,
                last_notified_at   = EXCLUDED.last_notified_at,
                next_notifiable_at = EXCLUDED.next_notifiable_at`,
		string(on.OrderID),
		on.NotifyCount,
		on.LastNotifiedAt,
		on.NextNotifiableAt,
	)
	return err
}
