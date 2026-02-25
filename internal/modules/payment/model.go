// README: Payment model, status, and method constants.
package payment

import (
	"errors"
	"time"
)

type PaymentMethod string

const (
	CreditCard PaymentMethod = "credit_card"
	Wallet     PaymentMethod = "wallet"
)

type PaymentStatus string

const (
	Pending PaymentStatus = "pending"
	Paid    PaymentStatus = "paid"
	Failed  PaymentStatus = "failed"
)

type Payment struct {
	PaymentID     int64         `db:"payment_id"`
	TripID        int64         `db:"trip_id"`
	PaymentMethod PaymentMethod `db:"payment_method"`
	Amount        float64       `db:"amount"`
	Status        PaymentStatus `db:"payment_status"`
	PaidAt        *time.Time    `db:"paid_at"`
	TransactionID *string       `db:"transaction_id"` // set after gateway charge
}

var (
	ErrNotFound   = errors.New("payment: not found")
	ErrBadRequest = errors.New("payment: bad request")
)
