// README: PaymentClient interface — internal API for the Order module to use Payment features.
package payment

import "context"

// PaymentClient defines the subset of payment operations exposed to the Order module.
type PaymentClient interface {
	CreatePayment(ctx context.Context, tripID int64, method PaymentMethod, amount float64) (*Payment, error)
	GetPaymentsByTrip(ctx context.Context, tripID int64) ([]*Payment, error)
}
