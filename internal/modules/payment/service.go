// README: Payment service implements CRUD for payments.
package payment

import (
	"context"
	"errors"
)

var (
	ErrNotFound   = errors.New("payment not found")
	ErrBadRequest = errors.New("bad request")
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

type CreateCommand struct {
	TripID        string
	PaymentMethod string // 'credit_card' or 'wallet'
	Amount        int64
	Currency      string
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) (int64, error) {
	if cmd.TripID == "" || cmd.PaymentMethod == "" || cmd.Amount <= 0 {
		return 0, ErrBadRequest
	}
	switch cmd.PaymentMethod {
	case "credit_card", "wallet":
	default:
		return 0, ErrBadRequest
	}
	currency := cmd.Currency
	if currency == "" {
		currency = "TWD"
	}
	return s.store.Create(ctx, &Payment{
		TripID:        cmd.TripID,
		PaymentMethod: cmd.PaymentMethod,
		Amount:        cmd.Amount,
		Currency:      currency,
		PaymentStatus: StatusPending,
	})
}

func (s *Service) Get(ctx context.Context, paymentID int64) (*Payment, error) {
	return s.store.Get(ctx, paymentID)
}

func (s *Service) UpdateStatus(ctx context.Context, paymentID int64, status Status) error {
	switch status {
	case StatusPaid, StatusPending, StatusFailed:
	default:
		return ErrBadRequest
	}
	return s.store.UpdateStatus(ctx, paymentID, status)
}
