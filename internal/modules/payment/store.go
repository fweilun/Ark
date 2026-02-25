// README: Payment store interface and PostgreSQL implementation.
package payment

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store defines the data access interface for payments.
type Store interface {
	CreatePayment(ctx context.Context, p *Payment) error
	GetPaymentByID(ctx context.Context, id int64) (*Payment, error)
	GetPaymentsByTripID(ctx context.Context, tripID int64) ([]*Payment, error)
	UpdatePaymentStatus(ctx context.Context, id int64, status PaymentStatus, paidAt *time.Time) error
	// Extensions needed for gateway integration.
	GetPaymentByTransactionID(ctx context.Context, txID string) (*Payment, error)
	SetTransactionID(ctx context.Context, id int64, txID string) error
}

type pgStore struct {
	db *pgxpool.Pool
}

// NewStore creates a PostgreSQL-backed Store.
func NewStore(db *pgxpool.Pool) Store {
	return &pgStore{db: db}
}

// CreatePayment inserts a new payment record (status: pending) and sets PaymentID from the DB.
func (s *pgStore) CreatePayment(ctx context.Context, p *Payment) error {
	row := s.db.QueryRow(ctx, `
        INSERT INTO payments (trip_id, payment_method, amount, payment_status)
        VALUES ($1, $2, $3, $4)
        RETURNING payment_id`,
		p.TripID,
		string(p.PaymentMethod),
		p.Amount,
		string(p.Status),
	)
	return row.Scan(&p.PaymentID)
}

// GetPaymentByID retrieves a payment by its primary key.
func (s *pgStore) GetPaymentByID(ctx context.Context, id int64) (*Payment, error) {
	row := s.db.QueryRow(ctx, `
        SELECT payment_id, trip_id, payment_method, amount, payment_status, paid_at, transaction_id
        FROM payments
        WHERE payment_id = $1`, id,
	)
	p, err := scanPayment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// GetPaymentsByTripID returns all payments associated with a trip.
func (s *pgStore) GetPaymentsByTripID(ctx context.Context, tripID int64) ([]*Payment, error) {
	rows, err := s.db.Query(ctx, `
        SELECT payment_id, trip_id, payment_method, amount, payment_status, paid_at, transaction_id
        FROM payments
        WHERE trip_id = $1
        ORDER BY payment_id ASC`, tripID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*Payment
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return payments, nil
}

// UpdatePaymentStatus updates the status and paid_at timestamp of a payment.
func (s *pgStore) UpdatePaymentStatus(ctx context.Context, id int64, status PaymentStatus, paidAt *time.Time) error {
	tag, err := s.db.Exec(ctx, `
        UPDATE payments
        SET payment_status = $1, paid_at = $2
        WHERE payment_id = $3`,
		string(status), paidAt, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetPaymentByTransactionID finds a payment by the gateway-assigned transaction ID.
func (s *pgStore) GetPaymentByTransactionID(ctx context.Context, txID string) (*Payment, error) {
	row := s.db.QueryRow(ctx, `
        SELECT payment_id, trip_id, payment_method, amount, payment_status, paid_at, transaction_id
        FROM payments
        WHERE transaction_id = $1`, txID,
	)
	p, err := scanPayment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// SetTransactionID stores the gateway transaction ID on a payment record.
func (s *pgStore) SetTransactionID(ctx context.Context, id int64, txID string) error {
	tag, err := s.db.Exec(ctx, `
        UPDATE payments SET transaction_id = $1 WHERE payment_id = $2`,
		txID, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// rowScanner abstracts pgx.Row and pgx.Rows so scanPayment works for both.
type rowScanner interface {
	Scan(...any) error
}

func scanPayment(row rowScanner) (*Payment, error) {
	var p Payment
	var txID *string
	var paidAt *time.Time
	err := row.Scan(
		&p.PaymentID,
		&p.TripID,
		&p.PaymentMethod,
		&p.Amount,
		&p.Status,
		&paidAt,
		&txID,
	)
	if err != nil {
		return nil, err
	}
	p.PaidAt = paidAt
	p.TransactionID = txID
	return &p, nil
}
