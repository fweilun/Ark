// README: Payment service — business logic, gateway abstraction, and async processing.
package payment

import (
	"context"
	"fmt"
	"log"
	"time"
)

// PaymentGateway abstracts external payment providers.
type PaymentGateway interface {
	Charge(ctx context.Context, req *ChargeRequest) (*ChargeResponse, error)
}

// ChargeRequest is sent to the payment gateway.
type ChargeRequest struct {
	Amount float64
	Method PaymentMethod
	// Additional fields (e.g. card token, wallet ID) can be added as needed.
}

// ChargeResponse is returned by the payment gateway.
type ChargeResponse struct {
	TransactionID string
	Success       bool
	PaidAt        time.Time
	ErrorMessage  string
}

// Service defines the business operations for the payment module.
type Service interface {
	// CreatePayment records a pending payment and kicks off async processing.
	CreatePayment(ctx context.Context, tripID int64, method PaymentMethod, amount float64) (*Payment, error)

	// ProcessPayment calls the gateway and updates the payment status.
	ProcessPayment(ctx context.Context, paymentID int64) error

	// GetPayment returns a single payment by ID.
	GetPayment(ctx context.Context, paymentID int64) (*Payment, error)

	// GetPaymentsByTrip returns all payments for a trip.
	GetPaymentsByTrip(ctx context.Context, tripID int64) ([]*Payment, error)

	// HandleCallback processes an asynchronous webhook from the payment gateway.
	HandleCallback(ctx context.Context, callbackData map[string]interface{}) error
}

type serviceImpl struct {
	store   Store
	gateway PaymentGateway
}

// NewService creates a Service backed by the given Store and PaymentGateway.
func NewService(store Store, gateway PaymentGateway) Service {
	return &serviceImpl{store: store, gateway: gateway}
}

// CreatePayment creates a pending payment record and asynchronously calls ProcessPayment.
func (s *serviceImpl) CreatePayment(ctx context.Context, tripID int64, method PaymentMethod, amount float64) (*Payment, error) {
	if tripID <= 0 {
		return nil, ErrBadRequest
	}
	if amount <= 0 {
		return nil, ErrBadRequest
	}
	if method != CreditCard && method != Wallet {
		return nil, ErrBadRequest
	}

	p := &Payment{
		TripID:        tripID,
		PaymentMethod: method,
		Amount:        amount,
		Status:        Pending,
	}
	if err := s.store.CreatePayment(ctx, p); err != nil {
		return nil, err
	}

	// Kick off payment processing asynchronously. Use a detached context so
	// cancellation of the HTTP request does not abort the payment flow.
	go func(id int64) {
		if err := s.ProcessPayment(context.Background(), id); err != nil {
			log.Printf("payment: async ProcessPayment(%d) error: %v", id, err)
		}
	}(p.PaymentID)

	return p, nil
}

// ProcessPayment calls the payment gateway and updates the payment record.
func (s *serviceImpl) ProcessPayment(ctx context.Context, paymentID int64) error {
	p, err := s.store.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return err
	}

	resp, err := s.gateway.Charge(ctx, &ChargeRequest{
		Amount: p.Amount,
		Method: p.PaymentMethod,
	})
	if err != nil {
		// Gateway returned a transport-level error; mark payment failed.
		_ = s.store.UpdatePaymentStatus(ctx, paymentID, Failed, nil)
		return fmt.Errorf("payment: gateway charge error: %w", err)
	}

	if resp.Success {
		if err := s.store.SetTransactionID(ctx, paymentID, resp.TransactionID); err != nil {
			return err
		}
		t := resp.PaidAt
		return s.store.UpdatePaymentStatus(ctx, paymentID, Paid, &t)
	}

	return s.store.UpdatePaymentStatus(ctx, paymentID, Failed, nil)
}

// GetPayment returns a payment by ID.
func (s *serviceImpl) GetPayment(ctx context.Context, paymentID int64) (*Payment, error) {
	return s.store.GetPaymentByID(ctx, paymentID)
}

// GetPaymentsByTrip returns all payments for the given trip.
func (s *serviceImpl) GetPaymentsByTrip(ctx context.Context, tripID int64) ([]*Payment, error) {
	return s.store.GetPaymentsByTripID(ctx, tripID)
}

// HandleCallback processes a webhook from the payment gateway.
// It validates the callback, finds the payment by transaction ID, and updates its status.
func (s *serviceImpl) HandleCallback(ctx context.Context, callbackData map[string]interface{}) error {
	txID, ok := callbackData["transaction_id"].(string)
	if !ok || txID == "" {
		return ErrBadRequest
	}

	success, _ := callbackData["success"].(bool)

	p, err := s.store.GetPaymentByTransactionID(ctx, txID)
	if err != nil {
		return err
	}

	if success {
		var paidAt *time.Time
		if ts, ok := callbackData["paid_at"].(string); ok && ts != "" {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				paidAt = &t
			}
		}
		if paidAt == nil {
			now := time.Now()
			paidAt = &now
		}
		return s.store.UpdatePaymentStatus(ctx, p.PaymentID, Paid, paidAt)
	}

	return s.store.UpdatePaymentStatus(ctx, p.PaymentID, Failed, nil)
}
