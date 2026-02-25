// README: Payment model and status definitions.
package payment

import "time"

type Status string

const (
	StatusPaid    Status = "paid"
	StatusPending Status = "pending"
	StatusFailed  Status = "failed"
)

// Payment records a financial transaction associated with a trip.
type Payment struct {
	PaymentID     int64
	TripID        string // FK to orders.id
	PaymentMethod string // 'credit_card' or 'wallet'
	Amount        int64
	Currency      string
	PaymentStatus Status
	PaidAt        *time.Time
}
