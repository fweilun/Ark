// README: Matching service orchestrates candidate pools and triggers order matching.
package matching

import (
	"context"
	"math/rand/v2"
	"time"

	"ark/internal/config"
	"ark/internal/modules/order"
	"ark/internal/types"
)

const (
	// maxDriversToConsider is the pool size randomly sampled from all nearby drivers.
	maxDriversToConsider = 10
	// maxDriversToNotify is the number of drivers to push notifications to per match cycle.
	maxDriversToNotify = 5
	// instantNotifiedTTL is the retry window for instant (waiting) orders.
	instantNotifiedTTL = 2 * time.Minute
	// scheduledInitialPushTTL is the proactive push window before a scheduled order goes fully public.
	scheduledInitialPushTTL = 10 * time.Minute
)

// OrderMatcher transitions an order to the matched/approaching state.
type OrderMatcher interface {
	Match(ctx context.Context, cmd order.MatchCommand) error
}

// OrderLister queries orders that require driver matching.
type OrderLister interface {
	ListWaiting(ctx context.Context) ([]*order.Order, error)
	ListScheduledOpen(ctx context.Context) ([]*order.Order, error)
}

// Notifier delivers push notifications to individual drivers.
// Implementations may use FCM or any other push channel.
// A nil Notifier silently skips notifications (useful in tests).
type Notifier interface {
	NotifyDriverNewOrder(ctx context.Context, driverID types.ID, info NotifyInfo) error
}

type Service struct {
	store    *Store
	order    OrderMatcher
	orders   OrderLister
	notifier Notifier // may be nil
	cfg      config.MatchingConfig
}

func NewService(store *Store, order OrderMatcher, orders OrderLister, notifier Notifier, cfg config.MatchingConfig) *Service {
	return &Service{store: store, order: order, orders: orders, notifier: notifier, cfg: cfg}
}

func (s *Service) AddCandidate(ctx context.Context, c Candidate) error {
	return s.store.AddCandidate(ctx, c)
}

func (s *Service) RemoveCandidate(ctx context.Context, id types.ID, t CandidateType) error {
	return s.store.RemoveCandidate(ctx, id)
}

func (s *Service) TryImmediateMatch(ctx context.Context, c Candidate) error {
	if c.Type != CandidatePassenger {
		return nil
	}
	// Best-effort: find nearby drivers and notify them immediately.
	drivers, err := s.store.NearbyDrivers(ctx, c.Position, s.cfg.RadiusKm)
	if err != nil || len(drivers) == 0 {
		return nil
	}
	selected := selectRandom(drivers, maxDriversToConsider)
	toNotify := selected
	if len(toNotify) > maxDriversToNotify {
		toNotify = selected[:maxDriversToNotify]
	}
	if s.notifier != nil {
		for _, driverID := range toNotify {
			_ = s.notifier.NotifyDriverNewOrder(ctx, driverID, NotifyInfo{
				OrderID:      c.ID,
				PickupLat:    c.Position.Lat,
				PickupLng:    c.Position.Lng,
				EstimatedFee: 0,
			})
		}
	}
	return nil
}

func (s *Service) RunScheduler(ctx context.Context) {
	tick := time.Duration(s.cfg.TickSeconds) * time.Second
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runMatchCycle(ctx)
		}
	}
}

func (s *Service) runMatchCycle(ctx context.Context) {
	if s.orders == nil {
		return
	}
	s.matchWaitingOrders(ctx)
	s.matchScheduledOrders(ctx)
}

// matchWaitingOrders finds all instant orders in StatusWaiting and pushes them to nearby drivers.
func (s *Service) matchWaitingOrders(ctx context.Context) {
	orders, err := s.orders.ListWaiting(ctx)
	if err != nil {
		return
	}
	for _, o := range orders {
		already, _ := s.store.IsNotified(ctx, o.ID)
		if already {
			continue
		}
		s.pushOrderToDrivers(ctx, o, instantNotifiedTTL)
	}
}

// matchScheduledOrders finds open scheduled orders and proactively pushes them to nearby drivers.
// The push is gated by IsNotified so each order is pushed at most once per scheduledInitialPushTTL window.
func (s *Service) matchScheduledOrders(ctx context.Context) {
	orders, err := s.orders.ListScheduledOpen(ctx)
	if err != nil {
		return
	}
	for _, o := range orders {
		already, _ := s.store.IsNotified(ctx, o.ID)
		if already {
			continue
		}
		s.pushOrderToDrivers(ctx, o, scheduledInitialPushTTL)
	}
}

// pushOrderToDrivers finds nearby drivers, randomly selects up to maxDriversToConsider,
// notifies up to maxDriversToNotify of them, and marks the order as notified.
func (s *Service) pushOrderToDrivers(ctx context.Context, o *order.Order, notifiedTTL time.Duration) {
	drivers, err := s.store.NearbyDrivers(ctx, o.Pickup, s.cfg.RadiusKm)
	if err != nil || len(drivers) == 0 {
		return
	}

	// Randomly select up to maxDriversToConsider from all nearby drivers.
	selected := selectRandom(drivers, maxDriversToConsider)

	// Notify up to maxDriversToNotify of the selected set.
	toNotify := selected
	if len(toNotify) > maxDriversToNotify {
		toNotify = selected[:maxDriversToNotify]
	}

	if s.notifier != nil {
		for _, driverID := range toNotify {
			_ = s.notifier.NotifyDriverNewOrder(ctx, driverID, NotifyInfo{
				OrderID:      o.ID,
				PickupLat:    o.Pickup.Lat,
				PickupLng:    o.Pickup.Lng,
				DropoffLat:   o.Dropoff.Lat,
				DropoffLng:   o.Dropoff.Lng,
				EstimatedFee: o.EstimatedFee.Amount,
			})
		}
	}

	// Mark as notified even when notifier is nil to avoid a busy-retry loop.
	_ = s.store.MarkNotified(ctx, o.ID, notifiedTTL)
}

// selectRandom randomly selects up to n items from ids using a partial Fisher-Yates shuffle.
// If len(ids) <= n, a copy of ids is returned unchanged.
func selectRandom(ids []types.ID, n int) []types.ID {
	result := make([]types.ID, len(ids))
	copy(result, ids)
	if len(result) <= n {
		return result
	}
	// Partial Fisher-Yates: shuffle only the tail so that result[0..n-1]
	// is a uniform random sample without replacement.
	for i := len(result) - 1; i >= n; i-- {
		j := rand.IntN(i + 1)
		result[i], result[j] = result[j], result[i]
	}
	return result[:n]
}
