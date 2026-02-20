// README: Matching service orchestrates candidate pools and triggers order matching.
package matching

import (
"context"
"log"
"math/rand"
"time"

"ark/internal/config"
"ark/internal/modules/order"
"ark/internal/types"
)

// OrderMatcher is the interface the matching service uses to interact with the order module.
type OrderMatcher interface {
Match(ctx context.Context, cmd order.MatchCommand) error
Get(ctx context.Context, id types.ID) (*order.Order, error)
ListAvailableScheduled(ctx context.Context, from, to time.Time) ([]*order.Order, error)
}

// MatchingStore is the interface for the matching backing store (Redis).
type MatchingStore interface {
AddCandidate(ctx context.Context, c Candidate) error
RemoveCandidate(ctx context.Context, id types.ID) error
NearbyDrivers(ctx context.Context, p types.Point, radiusKm float64) ([]types.ID, error)
RecordDispatch(ctx context.Context, orderID types.ID, driverIDs []types.ID) error
GetDispatchedAt(ctx context.Context, orderID types.ID) (time.Time, bool, error)
MarkOrderBroadcast(ctx context.Context, orderID types.ID) error
IsOrderBroadcast(ctx context.Context, orderID types.ID) (bool, error)
}

// Notifier sends push notifications to drivers about available orders.
// Implementations may use FCM, APNs, or any other push mechanism.
type Notifier interface {
NotifyDriver(ctx context.Context, driverID types.ID, orderID types.ID) error
}

type Service struct {
store    MatchingStore
order    OrderMatcher
cfg      config.MatchingConfig
notifier Notifier
}

func NewService(store MatchingStore, order OrderMatcher, cfg config.MatchingConfig) *Service {
return &Service{store: store, order: order, cfg: cfg}
}

// SetNotifier attaches a push-notification backend. If not set, notifications are skipped.
func (s *Service) SetNotifier(n Notifier) {
s.notifier = n
}

// PickRandomDrivers randomly selects up to n driver IDs from pool using a Fisher-Yates shuffle.
// If len(pool) <= n, all drivers are returned in shuffled order. The original pool is not modified.
//
// Note: uses math/rand, which in Go 1.20+ is automatically seeded with a random value at program
// start. No additional seeding is required.
func PickRandomDrivers(pool []types.ID, n int) []types.ID {
if n <= 0 || len(pool) == 0 {
return nil
}
cp := make([]types.ID, len(pool))
copy(cp, pool)
for i := len(cp) - 1; i > 0; i-- {
j := rand.Intn(i + 1)
cp[i], cp[j] = cp[j], cp[i]
}
if n > len(cp) {
return cp
}
return cp[:n]
}

func (s *Service) AddCandidate(ctx context.Context, c Candidate) error {
return s.store.AddCandidate(ctx, c)
}

func (s *Service) RemoveCandidate(ctx context.Context, id types.ID, t CandidateType) error {
return s.store.RemoveCandidate(ctx, id)
}

func (s *Service) TryImmediateMatch(ctx context.Context, c Candidate) error {
nearby, err := s.store.NearbyDrivers(ctx, c.Position, s.cfg.RadiusKm)
if err != nil {
return err
}
drivers := PickRandomDrivers(nearby, 1)
if len(drivers) == 0 {
return nil
}
return s.order.Match(ctx, order.MatchCommand{
OrderID:   c.ID,
DriverID:  drivers[0],
MatchedAt: time.Now(),
})
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
s.tickScheduledMatching(ctx)
}
}
}

// tickScheduledMatching processes all open scheduled orders in the lookahead window.
func (s *Service) tickScheduledMatching(ctx context.Context) {
now := time.Now()
orders, err := s.order.ListAvailableScheduled(ctx, now, now.Add(scheduledOrderLookaheadWindow))
if err != nil {
log.Printf("matching: list available scheduled orders: %v", err)
return
}
for _, o := range orders {
s.processScheduledOrder(ctx, o, now)
}
}

// processScheduledOrder applies the T0 / T30s / T-24h dispatch rules for a single order.
func (s *Service) processScheduledOrder(ctx context.Context, o *order.Order, now time.Time) {
dispatchedAt, dispatched, err := s.store.GetDispatchedAt(ctx, o.ID)
if err != nil {
log.Printf("matching: get dispatch state for order %s: %v", o.ID, err)
return
}

if !dispatched {
// T0: first encounter — dispatch to the initial driver pool.
s.dispatchInitial(ctx, o)
return
}

broadcast, err := s.store.IsOrderBroadcast(ctx, o.ID)
if err != nil {
log.Printf("matching: check broadcast state for order %s: %v", o.ID, err)
return
}
if broadcast {
return
}

// T-24h: if within one day of pickup and not yet broadcast, widen the notification pool.
if o.ScheduledAt != nil && o.ScheduledAt.Sub(now) <= oneDayBefore {
s.broadcastWider(ctx, o)
_ = s.store.MarkOrderBroadcast(ctx, o.ID)
return
}

// T30s: if the order has been waiting without a claim for broadcastDelay, open to all.
if now.Sub(dispatchedAt) >= broadcastDelay {
_ = s.store.MarkOrderBroadcast(ctx, o.ID)
// No further driver notification needed — the order is already visible via
// ListAvailableScheduled; drivers will see it on their next poll.
}
}

// dispatchInitial picks a random pool of nearby drivers, notifies a subset, and records the dispatch.
func (s *Service) dispatchInitial(ctx context.Context, o *order.Order) {
nearby, err := s.store.NearbyDrivers(ctx, o.Pickup, s.cfg.RadiusKm)
if err != nil {
log.Printf("matching: nearby drivers for order %s: %v", o.ID, err)
return
}
if len(nearby) == 0 {
return
}
pool := PickRandomDrivers(nearby, selectPoolSize)
toNotify := PickRandomDrivers(pool, notifyInitialCount)

_ = s.store.RecordDispatch(ctx, o.ID, toNotify)
s.notifyDrivers(ctx, toNotify, o.ID)
}

// broadcastWider notifies a wider pool of drivers (T-24h path).
func (s *Service) broadcastWider(ctx context.Context, o *order.Order) {
nearby, err := s.store.NearbyDrivers(ctx, o.Pickup, s.cfg.RadiusKm*2)
if err != nil {
log.Printf("matching: nearby drivers (wider) for order %s: %v", o.ID, err)
return
}
if len(nearby) == 0 {
return
}
toNotify := PickRandomDrivers(nearby, broadcastExtraCount)
s.notifyDrivers(ctx, toNotify, o.ID)
}

// notifyDrivers sends push notifications to each driver if a Notifier is configured.
func (s *Service) notifyDrivers(ctx context.Context, driverIDs []types.ID, orderID types.ID) {
if s.notifier == nil {
return
}
for _, driverID := range driverIDs {
_ = s.notifier.NotifyDriver(ctx, driverID, orderID)
}
}
