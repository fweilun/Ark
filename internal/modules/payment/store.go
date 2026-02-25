// README: Payment store backed by PostgreSQL.
package payment

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, p *Payment) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO payments (trip_id, payment_method, amount, currency, payment_status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING payment_id`,
		p.TripID, p.PaymentMethod, p.Amount, p.Currency, string(p.PaymentStatus),
	).Scan(&id)
	return id, err
}

func (s *Store) Get(ctx context.Context, paymentID int64) (*Payment, error) {
	row := s.db.QueryRow(ctx, `
		SELECT payment_id, trip_id, payment_method, amount, currency, payment_status, paid_at
		FROM payments WHERE payment_id = $1`, paymentID,
	)
	var p Payment
	var paidAt sql.NullTime
	err := row.Scan(&p.PaymentID, &p.TripID, &p.PaymentMethod, &p.Amount, &p.Currency, &p.PaymentStatus, &paidAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if paidAt.Valid {
		t := paidAt.Time
		p.PaidAt = &t
	}
	return &p, nil
}

func (s *Store) UpdateStatus(ctx context.Context, paymentID int64, status Status) error {
	var paidAt *time.Time
	if status == StatusPaid {
		now := time.Now()
		paidAt = &now
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE payments SET payment_status = $1, paid_at = COALESCE($2, paid_at)
		WHERE payment_id = $3`,
		string(status), paidAt, paymentID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
