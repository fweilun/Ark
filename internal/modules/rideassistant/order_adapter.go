// README: Adapter bridging order.Service to rideassistant.OrderCreator interface.
package rideassistant

import (
	"context"

	"ark/internal/modules/order"
	"ark/internal/types"
)

// OrderServiceAdapter implements OrderCreator using the real order.Service.
type OrderServiceAdapter struct {
	svc *order.Service
}

// NewOrderServiceAdapter wraps an order.Service as an OrderCreator.
func NewOrderServiceAdapter(svc *order.Service) *OrderServiceAdapter {
	return &OrderServiceAdapter{svc: svc}
}

func (a *OrderServiceAdapter) Create(ctx context.Context, cmd CreateOrderCommand) (types.ID, error) {
	return a.svc.Create(ctx, order.CreateCommand{
		PassengerID: cmd.PassengerID,
		Pickup:      cmd.Pickup,
		Dropoff:     cmd.Dropoff,
		RideType:    cmd.RideType,
	})
}

func (a *OrderServiceAdapter) CreateScheduled(ctx context.Context, cmd CreateScheduledOrderCommand) (types.ID, error) {
	return a.svc.CreateScheduled(ctx, order.CreateScheduledCommand{
		PassengerID:        cmd.PassengerID,
		Pickup:             cmd.Pickup,
		Dropoff:            cmd.Dropoff,
		RideType:           cmd.RideType,
		ScheduledAt:        cmd.ScheduledAt,
		ScheduleWindowMins: cmd.ScheduleWindowMins,
	})
}
