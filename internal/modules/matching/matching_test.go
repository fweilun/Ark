// README: Matching service unit tests covering PickRandomDrivers and dispatch logic.
package matching

import (
"context"
"fmt"
"sync"
"testing"
"time"

"ark/internal/config"
"ark/internal/modules/order"
"ark/internal/types"
)

// ---------------------------------------------------------------------------
// Unit tests: PickRandomDrivers (pure function, no external dependencies)
// ---------------------------------------------------------------------------

func TestPickRandomDrivers_NormalCase(t *testing.T) {
pool := makeDriverPool(10)
selected := PickRandomDrivers(pool, 5)
if len(selected) != 5 {
t.Fatalf("expected 5, got %d", len(selected))
}
assertSubset(t, pool, selected)
assertUnique(t, selected)
}

func TestPickRandomDrivers_FewerThanN(t *testing.T) {
pool := makeDriverPool(3)
selected := PickRandomDrivers(pool, 10)
if len(selected) != 3 {
t.Fatalf("expected all 3, got %d", len(selected))
}
assertUnique(t, selected)
}

func TestPickRandomDrivers_ExactN(t *testing.T) {
pool := makeDriverPool(5)
selected := PickRandomDrivers(pool, 5)
if len(selected) != 5 {
t.Fatalf("expected 5, got %d", len(selected))
}
assertSubset(t, pool, selected)
assertUnique(t, selected)
}

func TestPickRandomDrivers_EmptyPool(t *testing.T) {
selected := PickRandomDrivers(nil, 5)
if len(selected) != 0 {
t.Fatalf("expected 0 from nil pool, got %d", len(selected))
}
selected = PickRandomDrivers([]types.ID{}, 5)
if len(selected) != 0 {
t.Fatalf("expected 0 from empty pool, got %d", len(selected))
}
}

func TestPickRandomDrivers_ZeroN(t *testing.T) {
pool := makeDriverPool(5)
selected := PickRandomDrivers(pool, 0)
if len(selected) != 0 {
t.Fatalf("expected 0 for n=0, got %d", len(selected))
}
}

func TestPickRandomDrivers_NegativeN(t *testing.T) {
pool := makeDriverPool(5)
selected := PickRandomDrivers(pool, -1)
if len(selected) != 0 {
t.Fatalf("expected 0 for n<0, got %d", len(selected))
}
}

func TestPickRandomDrivers_DoesNotMutatePool(t *testing.T) {
pool := makeDriverPool(5)
orig := make([]types.ID, len(pool))
copy(orig, pool)
PickRandomDrivers(pool, 3)
for i, d := range pool {
if d != orig[i] {
t.Fatalf("pool mutated at index %d: got %s, want %s", i, d, orig[i])
}
}
}

// TestPickRandomDrivers_Distribution verifies that over many runs each driver is selected
// with roughly uniform probability (no driver monopolises the selections).
func TestPickRandomDrivers_Distribution(t *testing.T) {
pool := makeDriverPool(10)
counts := make(map[types.ID]int, len(pool))
const runs = 1000
const pick = 5
for i := 0; i < runs; i++ {
for _, d := range PickRandomDrivers(pool, pick) {
counts[d]++
}
}
// Each driver should appear roughly runs*pick/len(pool) = 500 times.
// Allow generous bounds (±60%) to avoid flakiness.
expected := runs * pick / len(pool)
lo, hi := expected*40/100, expected*160/100
for _, d := range pool {
c := counts[d]
if c < lo || c > hi {
t.Errorf("driver %s appeared %d times, want roughly %d (+/-60%%)", d, c, expected)
}
}
}

// ---------------------------------------------------------------------------
// Integration tests: dispatch logic with in-memory mock store / order service
// ---------------------------------------------------------------------------

// mockMatchingStore is an in-memory MatchingStore for testing.
type mockMatchingStore struct {
mu         sync.Mutex
dispatched map[types.ID]time.Time
broadcast  map[types.ID]bool
drivers    []types.ID
}

func newMockMatchingStore(drivers []types.ID) *mockMatchingStore {
return &mockMatchingStore{
dispatched: make(map[types.ID]time.Time),
broadcast:  make(map[types.ID]bool),
drivers:    drivers,
}
}

func (m *mockMatchingStore) AddCandidate(_ context.Context, _ Candidate) error { return nil }
func (m *mockMatchingStore) RemoveCandidate(_ context.Context, _ types.ID) error { return nil }
func (m *mockMatchingStore) NearbyDrivers(_ context.Context, _ types.Point, _ float64) ([]types.ID, error) {
m.mu.Lock()
defer m.mu.Unlock()
cp := make([]types.ID, len(m.drivers))
copy(cp, m.drivers)
return cp, nil
}
func (m *mockMatchingStore) RecordDispatch(_ context.Context, orderID types.ID, _ []types.ID) error {
m.mu.Lock()
defer m.mu.Unlock()
m.dispatched[orderID] = time.Now()
return nil
}
func (m *mockMatchingStore) GetDispatchedAt(_ context.Context, orderID types.ID) (time.Time, bool, error) {
m.mu.Lock()
defer m.mu.Unlock()
t, ok := m.dispatched[orderID]
return t, ok, nil
}
func (m *mockMatchingStore) MarkOrderBroadcast(_ context.Context, orderID types.ID) error {
m.mu.Lock()
defer m.mu.Unlock()
m.broadcast[orderID] = true
return nil
}
func (m *mockMatchingStore) IsOrderBroadcast(_ context.Context, orderID types.ID) (bool, error) {
m.mu.Lock()
defer m.mu.Unlock()
return m.broadcast[orderID], nil
}
func (m *mockMatchingStore) isBroadcast(id types.ID) bool {
m.mu.Lock()
defer m.mu.Unlock()
return m.broadcast[id]
}
func (m *mockMatchingStore) isDispatched(id types.ID) bool {
m.mu.Lock()
defer m.mu.Unlock()
_, ok := m.dispatched[id]
return ok
}
func (m *mockMatchingStore) forceDispatchedAt(id types.ID, t time.Time) {
m.mu.Lock()
defer m.mu.Unlock()
m.dispatched[id] = t
}

// mockOrderService is an in-memory OrderMatcher for dispatch logic tests.
type mockOrderService struct {
mu     sync.Mutex
orders map[types.ID]*order.Order
}

func newMockOrderService() *mockOrderService {
return &mockOrderService{orders: make(map[types.ID]*order.Order)}
}

func (m *mockOrderService) addScheduledOrder(id types.ID, scheduledAt time.Time) {
m.mu.Lock()
defer m.mu.Unlock()
m.orders[id] = &order.Order{
ID:          id,
PassengerID: "p1",
Status:      order.StatusScheduled,
OrderType:   "scheduled",
ScheduledAt: &scheduledAt,
Pickup:      types.Point{Lat: 25.033, Lng: 121.565},
}
}
func (m *mockOrderService) claimOrder(id types.ID) {
m.mu.Lock()
defer m.mu.Unlock()
if o, ok := m.orders[id]; ok {
o.Status = order.StatusAssigned
}
}
func (m *mockOrderService) Match(_ context.Context, _ order.MatchCommand) error { return nil }
func (m *mockOrderService) Get(_ context.Context, id types.ID) (*order.Order, error) {
m.mu.Lock()
defer m.mu.Unlock()
o, ok := m.orders[id]
if !ok {
return nil, order.ErrNotFound
}
cp := *o
return &cp, nil
}
func (m *mockOrderService) ListAvailableScheduled(_ context.Context, _, _ time.Time) ([]*order.Order, error) {
m.mu.Lock()
defer m.mu.Unlock()
var result []*order.Order
for _, o := range m.orders {
if o.Status == order.StatusScheduled {
cp := *o
result = append(result, &cp)
}
}
return result, nil
}

func newTestCfg() config.MatchingConfig {
return config.MatchingConfig{TickSeconds: 3, RadiusKm: 3.0}
}

// ---------------------------------------------------------------------------
// Test: Normal flow — order dispatched on first tick, claimed, not broadcast.
// ---------------------------------------------------------------------------

// TestMatchingNormalFlow verifies that a new scheduled order is dispatched to drivers on first
// tick, and once claimed it is no longer visible in available list and not broadcast.
func TestMatchingNormalFlow(t *testing.T) {
ctx := context.Background()
drivers := makeDriverPool(10)
mStore := newMockMatchingStore(drivers)
mOrder := newMockOrderService()

orderID := types.ID("order_normal")
mOrder.addScheduledOrder(orderID, time.Now().Add(2*time.Hour))

svc := NewService(mStore, mOrder, newTestCfg())

// First tick: order is new — should be dispatched.
svc.tickScheduledMatching(ctx)

if !mStore.isDispatched(orderID) {
t.Fatal("expected order to be dispatched after first tick")
}
if mStore.isBroadcast(orderID) {
t.Fatal("order should not be broadcast immediately after dispatch")
}

// Driver claims the order.
mOrder.claimOrder(orderID)

// Simulate passage of time past broadcastDelay.
mStore.forceDispatchedAt(orderID, time.Now().Add(-broadcastDelay-time.Second))

// Second tick: order is now claimed (StatusAssigned) — not in available list,
// broadcast flag must stay false.
svc.tickScheduledMatching(ctx)

if mStore.isBroadcast(orderID) {
t.Fatal("claimed order must not be broadcast to the public list")
}
}

// ---------------------------------------------------------------------------
// Test: T30s — unclaimed order is opened to all drivers after broadcastDelay.
// ---------------------------------------------------------------------------

func TestMatchingBroadcastAfterDelay(t *testing.T) {
ctx := context.Background()
drivers := makeDriverPool(10)
mStore := newMockMatchingStore(drivers)
mOrder := newMockOrderService()

orderID := types.ID("order_broadcast")
mOrder.addScheduledOrder(orderID, time.Now().Add(2*time.Hour))

svc := NewService(mStore, mOrder, newTestCfg())

// First tick: dispatch.
svc.tickScheduledMatching(ctx)
if !mStore.isDispatched(orderID) {
t.Fatal("expected dispatch on first tick")
}

// Simulate that broadcastDelay has elapsed without a claim.
mStore.forceDispatchedAt(orderID, time.Now().Add(-broadcastDelay-time.Second))

// Second tick: should trigger broadcast.
svc.tickScheduledMatching(ctx)

if !mStore.isBroadcast(orderID) {
t.Fatal("expected order to be broadcast after broadcastDelay elapsed")
}
}

// ---------------------------------------------------------------------------
// Test: T-24h — order within 24h of pickup gets wider broadcast.
// ---------------------------------------------------------------------------

func TestMatchingOneDayBroadcast(t *testing.T) {
ctx := context.Background()
drivers := makeDriverPool(10)
mStore := newMockMatchingStore(drivers)
mOrder := newMockOrderService()

orderID := types.ID("order_24h")
// Scheduled 12 hours from now — within the oneDayBefore window.
mOrder.addScheduledOrder(orderID, time.Now().Add(12*time.Hour))

svc := NewService(mStore, mOrder, newTestCfg())

// First tick: dispatch.
svc.tickScheduledMatching(ctx)

// Simulate dispatch happened a while back; order still unclaimed.
mStore.forceDispatchedAt(orderID, time.Now().Add(-time.Hour))

// Second tick: T-24h threshold met — should broadcast wider.
svc.tickScheduledMatching(ctx)

if !mStore.isBroadcast(orderID) {
t.Fatal("expected wider broadcast when within 24h of scheduled pickup")
}
}

// ---------------------------------------------------------------------------
// Test: Concurrent PickRandomDrivers — results are independent and correct.
// ---------------------------------------------------------------------------

func TestMatchingConcurrentPickRandom(t *testing.T) {
pool := makeDriverPool(20)
const goroutines = 8
var wg sync.WaitGroup
results := make(chan []types.ID, goroutines)

for i := 0; i < goroutines; i++ {
wg.Add(1)
go func() {
defer wg.Done()
results <- PickRandomDrivers(pool, 5)
}()
}
wg.Wait()
close(results)

for sel := range results {
if len(sel) != 5 {
t.Fatalf("expected 5, got %d", len(sel))
}
assertUnique(t, sel)
assertSubset(t, pool, sel)
}
}

// ---------------------------------------------------------------------------
// Test: 4 rejections before accept — order remains available until a driver claims it.
// ---------------------------------------------------------------------------

// TestMatchingFourRejectsThenAccept verifies that after 4 drivers see but do not claim a
// scheduled order, the order stays in StatusScheduled; the 5th driver can claim it; and
// once claimed, the order no longer appears in the available-scheduled list.
func TestMatchingFourRejectsThenAccept(t *testing.T) {
mOrder := newMockOrderService()
orderID := types.ID("order_rejects")
mOrder.addScheduledOrder(orderID, time.Now().Add(2*time.Hour))

ctx := context.Background()

// 4 drivers inspect but don't claim — order must stay available.
for i := 0; i < 4; i++ {
o, err := mOrder.Get(ctx, orderID)
if err != nil {
t.Fatalf("get order (pass %d): %v", i, err)
}
if o.Status != order.StatusScheduled {
t.Fatalf("expected StatusScheduled after pass %d, got %s", i, o.Status)
}
available, err := mOrder.ListAvailableScheduled(ctx, time.Now(), time.Now().Add(24*time.Hour))
if err != nil {
t.Fatalf("list available (pass %d): %v", i, err)
}
found := false
for _, a := range available {
if a.ID == orderID {
found = true
break
}
}
if !found {
t.Fatalf("order should be in available list after pass %d", i)
}
}

// 5th driver claims the order.
mOrder.claimOrder(orderID)

o, err := mOrder.Get(ctx, orderID)
if err != nil {
t.Fatalf("get order after accept: %v", err)
}
if o.Status != order.StatusAssigned {
t.Fatalf("expected StatusAssigned after accept, got %s", o.Status)
}

// Claimed order must no longer appear in the available list.
available, err := mOrder.ListAvailableScheduled(ctx, time.Now(), time.Now().Add(24*time.Hour))
if err != nil {
t.Fatalf("list available after accept: %v", err)
}
for _, a := range available {
if a.ID == orderID {
t.Fatal("claimed order must not appear in available scheduled list")
}
}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeDriverPool(n int) []types.ID {
pool := make([]types.ID, n)
for i := range pool {
pool[i] = types.ID(fmt.Sprintf("driver_%d", i))
}
return pool
}

func assertSubset(t *testing.T, pool, subset []types.ID) {
t.Helper()
set := make(map[types.ID]bool, len(pool))
for _, d := range pool {
set[d] = true
}
for _, d := range subset {
if !set[d] {
t.Errorf("selected driver %s not in pool", d)
}
}
}

func assertUnique(t *testing.T, ids []types.ID) {
t.Helper()
seen := make(map[types.ID]bool, len(ids))
for _, d := range ids {
if seen[d] {
t.Errorf("duplicate driver ID %s", d)
}
seen[d] = true
}
}
