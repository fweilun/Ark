// README: Store interface for Order module to enable testing with mocks
package order

import (
	"context"
	"time"

	"ark/internal/types"
)

// OrderStore defines the interface for order persistence operations
type OrderStore interface {
	// Basic CRUD operations
	Create(ctx context.Context, o *Order) error
	Get(ctx context.Context, id types.ID) (*Order, error)
	UpdateStatus(ctx context.Context, id types.ID, from, to Status, version int, driverID *types.ID) (bool, error)
	AppendEvent(ctx context.Context, e *Event) error

	// Query operations
	HasActiveByPassenger(ctx context.Context, passengerID types.ID) (bool, error)

	// Scheduled order operations
	CreateScheduled(ctx context.Context, o *Order) error
	ListScheduledByPassenger(ctx context.Context, passengerID types.ID) ([]*Order, error)
	ListAvailableScheduled(ctx context.Context, from, to time.Time) ([]*Order, error)
	ClaimScheduled(ctx context.Context, orderID, driverID types.ID, expectVersion int) (bool, error)
	ReopenScheduled(ctx context.Context, orderID types.ID, expectVersion int, bonus int64) (bool, error)

	// Background operations
	BumpIncentiveBonusForApproaching(ctx context.Context, bump int64) error
	ExpireOverdueScheduled(ctx context.Context) error

	// ListUrgentPendingOrders returns all scheduled and waiting orders that have not
	// yet passed their scheduled time, ordered by urgency (earliest first).
	ListUrgentPendingOrders(ctx context.Context) ([]*Order, error)
}

// Ensure Store implements OrderStore interface
var _ OrderStore = (*Store)(nil)
