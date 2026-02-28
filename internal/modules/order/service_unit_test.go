// README: Pure-unit tests for Order service using an in-memory mock store.
// These tests cover the business-logic paths that don't need a real DB,
// giving coverage to all previously 0% functions in service.go and schedule.go.
package order

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"ark/internal/types"
)

// ---------------------------------------------------------------------------
// conflictingMockStore always fails UpdateStatus (simulates lost optimistic lock)
// ---------------------------------------------------------------------------

type conflictingMockStore struct {
	*mockOrderStore
}

func (c *conflictingMockStore) UpdateStatus(_ context.Context, _ types.ID, _, _ Status, _ int, _ *types.ID) (bool, error) {
	return false, nil // always fails → triggers ErrConflict in applyTransition
}

// ---------------------------------------------------------------------------
// In-memory mock store
// ---------------------------------------------------------------------------

type mockOrderStore struct {
	mu        sync.Mutex
	orders    map[types.ID]*Order
	events    []*Event
	appendErr error // if set, AppendEvent returns this error
}

func newMockStore() *mockOrderStore {
	return &mockOrderStore{orders: make(map[types.ID]*Order)}
}

func (m *mockOrderStore) Create(_ context.Context, o *Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *o
	m.orders[o.ID] = &cp
	return nil
}

func (m *mockOrderStore) Get(_ context.Context, id types.ID) (*Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *o
	return &cp, nil
}

func (m *mockOrderStore) UpdateStatus(_ context.Context, id types.ID, from, to Status, version int, driverID *types.ID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[id]
	if !ok {
		return false, ErrNotFound
	}
	if o.Status != from || o.StatusVersion != version {
		return false, nil // optimistic lock failed
	}
	o.Status = to
	o.StatusVersion++
	if driverID != nil {
		o.DriverID = driverID
	}
	return true, nil
}

func (m *mockOrderStore) AppendEvent(_ context.Context, e *Event) error {
	if m.appendErr != nil {
		return m.appendErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}

func (m *mockOrderStore) HasActiveByPassenger(_ context.Context, passengerID types.ID) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, o := range m.orders {
		if o.PassengerID == passengerID {
			for _, s := range activeStatuses {
				if o.Status == s {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func (m *mockOrderStore) CreateScheduled(_ context.Context, o *Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *o
	m.orders[o.ID] = &cp
	return nil
}

func (m *mockOrderStore) ListScheduledByPassenger(_ context.Context, passengerID types.ID) ([]*Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Order
	for _, o := range m.orders {
		if o.PassengerID == passengerID && o.OrderType == "scheduled" {
			cp := *o
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *mockOrderStore) ListAvailableScheduled(_ context.Context, from, to time.Time) ([]*Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Order
	for _, o := range m.orders {
		if o.Status == StatusScheduled && o.ScheduledAt != nil &&
			!o.ScheduledAt.Before(from) && !o.ScheduledAt.After(to) {
			cp := *o
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *mockOrderStore) ClaimScheduled(_ context.Context, orderID, driverID types.ID, expectVersion int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[orderID]
	if !ok {
		return false, ErrNotFound
	}
	if o.Status != StatusScheduled || o.StatusVersion != expectVersion {
		return false, nil
	}
	o.Status = StatusAssigned
	o.StatusVersion++
	o.DriverID = &driverID
	return true, nil
}

func (m *mockOrderStore) ReopenScheduled(_ context.Context, orderID types.ID, expectVersion int, bonus int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[orderID]
	if !ok {
		return false, ErrNotFound
	}
	if o.Status != StatusAssigned || o.StatusVersion != expectVersion {
		return false, nil
	}
	o.Status = StatusScheduled
	o.StatusVersion++
	o.DriverID = nil
	o.IncentiveBonus += bonus
	return true, nil
}

func (m *mockOrderStore) BumpIncentiveBonusForApproaching(_ context.Context, _ int64) error {
	return nil
}

func (m *mockOrderStore) ExpireOverdueScheduled(_ context.Context) error {
	return nil
}

// ---------------------------------------------------------------------------
// Mock pricing
// ---------------------------------------------------------------------------

type mockPricing struct {
	amount   int64
	currency string
	err      error
}

func (p *mockPricing) Estimate(_ context.Context, _ float64, _ string) (types.Money, error) {
	if p.err != nil {
		return types.Money{}, p.err
	}
	return types.Money{Amount: p.amount, Currency: p.currency}, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestSvc() (*Service, *mockOrderStore) {
	store := newMockStore()
	svc := NewService(store, nil)
	return svc, store
}

func makeOrder(store *mockOrderStore, passengerID types.ID, status Status) types.ID {
	id := newID()
	store.orders[id] = &Order{
		ID:            id,
		PassengerID:   passengerID,
		Status:        status,
		StatusVersion: 0,
		Pickup:        types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:       types.Point{Lat: 25.048, Lng: 121.532},
		RideType:      "economy",
		EstimatedFee:  types.Money{Amount: 15000, Currency: "TWD"},
		OrderType:     "instant",
		CreatedAt:     time.Now(),
	}
	return id
}

// ---------------------------------------------------------------------------
// Service.Create
// ---------------------------------------------------------------------------

func TestUnit_Create_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()

	id, err := svc.Create(ctx, CreateCommand{
		PassengerID: "pax-1",
		Pickup:      types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:     types.Point{Lat: 25.048, Lng: 121.532},
		RideType:    "economy",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty order ID")
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusWaiting {
		t.Errorf("expected status=waiting, got %s", o.Status)
	}
	if o.PassengerID != "pax-1" {
		t.Errorf("expected passengerID=pax-1, got %s", o.PassengerID)
	}
	if o.OrderType != "instant" {
		t.Errorf("expected orderType=instant, got %s", o.OrderType)
	}
}

func TestUnit_Create_MissingPassengerID(t *testing.T) {
	svc, _ := newTestSvc()
	_, err := svc.Create(context.Background(), CreateCommand{RideType: "economy"})
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

func TestUnit_Create_MissingRideType(t *testing.T) {
	svc, _ := newTestSvc()
	_, err := svc.Create(context.Background(), CreateCommand{PassengerID: "pax-1"})
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

func TestUnit_Create_BlockedByActiveOrder(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()

	// Seed an active order for the same passenger.
	makeOrder(store, "pax-blocked", StatusWaiting)

	_, err := svc.Create(ctx, CreateCommand{
		PassengerID: "pax-blocked",
		RideType:    "economy",
	})
	if !errors.Is(err, ErrActiveOrder) {
		t.Errorf("expected ErrActiveOrder, got %v", err)
	}
}

func TestUnit_Create_WithPricing(t *testing.T) {
	store := newMockStore()
	pricing := &mockPricing{amount: 18000, currency: "TWD"}
	svc := NewService(store, pricing)

	id, err := svc.Create(context.Background(), CreateCommand{
		PassengerID: "pax-pricing",
		Pickup:      types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:     types.Point{Lat: 25.048, Lng: 121.532},
		RideType:    "economy",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	o, _ := store.Get(context.Background(), id)
	if o.EstimatedFee.Amount != 18000 {
		t.Errorf("expected fee=18000, got %d", o.EstimatedFee.Amount)
	}
}

func TestUnit_Create_PricingErrorFallsBackToZero(t *testing.T) {
	store := newMockStore()
	pricing := &mockPricing{err: errors.New("pricing down")}
	svc := NewService(store, pricing)

	id, err := svc.Create(context.Background(), CreateCommand{
		PassengerID: "pax-no-price",
		RideType:    "economy",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	o, _ := store.Get(context.Background(), id)
	if o.EstimatedFee.Amount != 0 {
		t.Errorf("expected fee=0 on pricing error fallback, got %d", o.EstimatedFee.Amount)
	}
}

// ---------------------------------------------------------------------------
// Service.Get
// ---------------------------------------------------------------------------

func TestUnit_Get_NotFound(t *testing.T) {
	svc, _ := newTestSvc()
	_, err := svc.Get(context.Background(), "ghost")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUnit_Get_Found(t *testing.T) {
	svc, store := newTestSvc()
	id := makeOrder(store, "pax-get", StatusWaiting)

	o, err := svc.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if o.ID != id {
		t.Errorf("expected ID=%s, got %s", id, o.ID)
	}
}

// ---------------------------------------------------------------------------
// Service.Accept / Match / Depart
// ---------------------------------------------------------------------------

func TestUnit_Accept_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-accept", StatusWaiting)

	if err := svc.Accept(ctx, AcceptCommand{OrderID: id, DriverID: "drv-1"}); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusApproaching {
		t.Errorf("expected approaching, got %s", o.Status)
	}
	if o.DriverID == nil || *o.DriverID != "drv-1" {
		t.Errorf("expected driverID=drv-1, got %v", o.DriverID)
	}
}

func TestUnit_Accept_InvalidState(t *testing.T) {
	svc, store := newTestSvc()
	id := makeOrder(store, "pax-bad-accept", StatusApproaching)

	err := svc.Accept(context.Background(), AcceptCommand{OrderID: id, DriverID: "drv-x"})
	if !errors.Is(err, ErrInvalidState) {
		t.Errorf("expected ErrInvalidState, got %v", err)
	}
}

func TestUnit_Match_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-match", StatusWaiting)

	if err := svc.Match(ctx, MatchCommand{OrderID: id, DriverID: "drv-m", MatchedAt: time.Now()}); err != nil {
		t.Fatalf("Match: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusApproaching {
		t.Errorf("expected approaching, got %s", o.Status)
	}
}

func TestUnit_Depart_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()

	// Depart: Assigned → Approaching
	id := makeOrder(store, "pax-depart", StatusAssigned)

	if err := svc.Depart(ctx, DepartCommand{OrderID: id, DriverID: "drv-d"}); err != nil {
		t.Fatalf("Depart: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusApproaching {
		t.Errorf("expected approaching, got %s", o.Status)
	}
}

// ---------------------------------------------------------------------------
// Service.Arrive / Meet / Start
// ---------------------------------------------------------------------------

func TestUnit_Arrive_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-arrive", StatusApproaching)

	if err := svc.Arrive(ctx, ArriveCommand{OrderID: id}); err != nil {
		t.Fatalf("Arrive: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusArrived {
		t.Errorf("expected arrived, got %s", o.Status)
	}
}

func TestUnit_Meet_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-meet", StatusArrived)

	if err := svc.Meet(ctx, MeetCommand{OrderID: id}); err != nil {
		t.Fatalf("Meet: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusDriving {
		t.Errorf("expected driving, got %s", o.Status)
	}
}

func TestUnit_Start_DelegatesToMeet(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-start", StatusArrived)

	// Start() is an alias for Meet().
	if err := svc.Start(ctx, StartCommand{OrderID: id}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusDriving {
		t.Errorf("expected driving, got %s", o.Status)
	}
}

// ---------------------------------------------------------------------------
// Service.Complete / Pay
// ---------------------------------------------------------------------------

func TestUnit_Complete_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-complete", StatusDriving)

	if err := svc.Complete(ctx, CompleteCommand{OrderID: id}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusPayment {
		t.Errorf("expected payment, got %s", o.Status)
	}
}

func TestUnit_Pay_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-pay", StatusPayment)

	if err := svc.Pay(ctx, PayCommand{OrderID: id}); err != nil {
		t.Fatalf("Pay: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusComplete {
		t.Errorf("expected complete, got %s", o.Status)
	}
}

// ---------------------------------------------------------------------------
// Service.Cancel / Deny / Rematch
// ---------------------------------------------------------------------------

func TestUnit_Cancel_PassengerFromWaiting(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-cancel", StatusWaiting)

	if err := svc.Cancel(ctx, CancelCommand{OrderID: id, ActorType: "passenger", Reason: "changed_mind"}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusCancelled {
		t.Errorf("expected cancelled, got %s", o.Status)
	}
}

func TestUnit_Cancel_DriverFromApproaching(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-drv-cancel", StatusApproaching)

	if err := svc.Cancel(ctx, CancelCommand{OrderID: id, ActorType: "driver", Reason: "flat_tyre"}); err != nil {
		t.Fatalf("Cancel from approaching: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusCancelled {
		t.Errorf("expected cancelled, got %s", o.Status)
	}
}

func TestUnit_Cancel_TerminalStateRejected(t *testing.T) {
	svc, store := newTestSvc()
	id := makeOrder(store, "pax-terminal", StatusComplete)

	err := svc.Cancel(context.Background(), CancelCommand{OrderID: id, ActorType: "passenger"})
	if !errors.Is(err, ErrInvalidState) {
		t.Errorf("expected ErrInvalidState for cancel from terminal, got %v", err)
	}
}

func TestUnit_Deny_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-deny", StatusApproaching)

	if err := svc.Deny(ctx, DenyCommand{OrderID: id, DriverID: "drv-deny"}); err != nil {
		t.Fatalf("Deny: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusWaiting {
		t.Errorf("expected waiting after deny, got %s", o.Status)
	}
}

func TestUnit_Rematch_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-rematch", StatusApproaching)

	if err := svc.Rematch(ctx, RematchCommand{OrderID: id}); err != nil {
		t.Fatalf("Rematch: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusWaiting {
		t.Errorf("expected waiting after rematch, got %s", o.Status)
	}
}

// ---------------------------------------------------------------------------
// applyTransition edge cases (order not found, conflict from stale version)
// ---------------------------------------------------------------------------

func TestUnit_ApplyTransition_OrderNotFound(t *testing.T) {
	svc, _ := newTestSvc()
	err := svc.Cancel(context.Background(), CancelCommand{OrderID: "no-such-order", ActorType: "passenger"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUnit_ApplyTransition_Conflict(t *testing.T) {
	// Use a store that returns (false, nil) from UpdateStatus to simulate a lost race.
	conflictStore := &conflictingMockStore{mockOrderStore: newMockStore()}
	svc := NewService(conflictStore, nil)
	ctx := context.Background()

	id := makeOrder(conflictStore.mockOrderStore, "pax-conflict", StatusWaiting)

	// UpdateStatus will return (false, nil) → applyTransition must return ErrConflict.
	err := svc.Accept(ctx, AcceptCommand{OrderID: id, DriverID: "drv-x"})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict on stale version, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// distanceKm
// ---------------------------------------------------------------------------

func TestUnit_DistanceKm_Positive(t *testing.T) {
	// Taipei 101 → Taipei Main Station (~3-5 km)
	a := types.Point{Lat: 25.0340, Lng: 121.5645}
	b := types.Point{Lat: 25.0478, Lng: 121.5170}
	d := distanceKm(a, b)
	if d <= 0 {
		t.Errorf("expected positive distance, got %f", d)
	}
	if d > 20 {
		t.Errorf("distance seems too large for nearby Taipei points: %f km", d)
	}
}

func TestUnit_DistanceKm_SamePoint(t *testing.T) {
	p := types.Point{Lat: 25.0, Lng: 121.0}
	d := distanceKm(p, p)
	if d != 0 {
		t.Errorf("expected 0 for same point, got %f", d)
	}
}

// ---------------------------------------------------------------------------
// resolveActorID (via AppendEvent events inspection)
// ---------------------------------------------------------------------------

func TestUnit_ResolveActorID_PassengerActor(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-actor", StatusWaiting)

	_ = svc.Cancel(ctx, CancelCommand{OrderID: id, ActorType: "passenger"})

	store.mu.Lock()
	evts := store.events
	store.mu.Unlock()
	found := false
	for _, e := range evts {
		if e.OrderID == id && e.ActorType == "passenger" && e.ActorID != nil {
			found = true
		}
	}
	if !found {
		t.Error("expected event with actorType=passenger and non-nil actorID")
	}
}

func TestUnit_ResolveActorID_DriverActor(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-drv-actor", StatusWaiting)

	// Accept sets driverID, actorType=driver; resolveActorID should return the driver.
	_ = svc.Accept(ctx, AcceptCommand{OrderID: id, DriverID: "drv-actor"})

	store.mu.Lock()
	evts := store.events
	store.mu.Unlock()
	found := false
	for _, e := range evts {
		if e.OrderID == id && e.ActorType == "driver" && e.ActorID != nil {
			found = true
		}
	}
	if !found {
		t.Error("expected event with actorType=driver and non-nil actorID")
	}
}

func TestUnit_ResolveActorID_SystemActor(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()
	id := makeOrder(store, "pax-system-actor", StatusApproaching)

	// Rematch uses actorType="system"; resolveActorID returns nil for unknown actors.
	_ = svc.Rematch(ctx, RematchCommand{OrderID: id})

	store.mu.Lock()
	evts := store.events
	store.mu.Unlock()
	for _, e := range evts {
		if e.OrderID == id && e.ActorType == "system" {
			// actorID may be nil for system; just verify the event was recorded.
			return
		}
	}
	t.Error("expected event with actorType=system")
}

// ---------------------------------------------------------------------------
// AppendEvent (called internally by all state transitions)
// ---------------------------------------------------------------------------

func TestUnit_AppendEvent_CalledOnCreate(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()

	_, _ = svc.Create(ctx, CreateCommand{
		PassengerID: "pax-event",
		RideType:    "economy",
	})

	store.mu.Lock()
	n := len(store.events)
	store.mu.Unlock()
	if n == 0 {
		t.Error("expected at least one event after Create")
	}
	if store.events[0].FromStatus != StatusNone || store.events[0].ToStatus != StatusWaiting {
		t.Errorf("first event should be none→waiting, got %s→%s",
			store.events[0].FromStatus, store.events[0].ToStatus)
	}
}

func TestUnit_AppendEvent_ErrorIgnored(t *testing.T) {
	// AppendEvent errors must not bubble up; state transitions still succeed.
	store := newMockStore()
	store.appendErr = errors.New("event store down")
	svc := NewService(store, nil)
	ctx := context.Background()

	id, err := svc.Create(ctx, CreateCommand{
		PassengerID: "pax-no-event",
		RideType:    "economy",
	})
	if err != nil {
		t.Fatalf("Create must succeed even when AppendEvent fails: %v", err)
	}
	// State transition should also succeed despite event error.
	if err := svc.Accept(ctx, AcceptCommand{OrderID: id, DriverID: "drv-x"}); err != nil {
		t.Fatalf("Accept must succeed even when AppendEvent fails: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Full instant-order happy path (unit, no DB)
// ---------------------------------------------------------------------------

func TestUnit_FullInstantOrderFlow(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()

	// Create
	id, err := svc.Create(ctx, CreateCommand{
		PassengerID: "pax-full",
		Pickup:      types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:     types.Point{Lat: 25.048, Lng: 121.532},
		RideType:    "economy",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	steps := []struct {
		name string
		fn   func() error
		want Status
	}{
		{"Accept", func() error {
			return svc.Accept(ctx, AcceptCommand{OrderID: id, DriverID: "drv-full"})
		}, StatusApproaching},
		{"Arrive", func() error {
			return svc.Arrive(ctx, ArriveCommand{OrderID: id})
		}, StatusArrived},
		{"Meet", func() error {
			return svc.Meet(ctx, MeetCommand{OrderID: id})
		}, StatusDriving},
		{"Complete", func() error {
			return svc.Complete(ctx, CompleteCommand{OrderID: id})
		}, StatusPayment},
		{"Pay", func() error {
			return svc.Pay(ctx, PayCommand{OrderID: id})
		}, StatusComplete},
	}

	for _, step := range steps {
		if err := step.fn(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		o, _ := store.Get(ctx, id)
		if o.Status != step.want {
			t.Errorf("after %s: expected %s, got %s", step.name, step.want, o.Status)
		}
	}
}

// ---------------------------------------------------------------------------
// schedule.go — CreateScheduled
// ---------------------------------------------------------------------------

func TestUnit_CreateScheduled_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()

	scheduledAt := time.Now().Add(2 * time.Hour)
	id, err := svc.CreateScheduled(ctx, CreateScheduledCommand{
		PassengerID:        "pax-sched",
		Pickup:             types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:            types.Point{Lat: 25.048, Lng: 121.532},
		RideType:           "economy",
		ScheduledAt:        scheduledAt,
		ScheduleWindowMins: 30,
	})
	if err != nil {
		t.Fatalf("CreateScheduled: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusScheduled {
		t.Errorf("expected scheduled, got %s", o.Status)
	}
	if o.OrderType != "scheduled" {
		t.Errorf("expected orderType=scheduled, got %s", o.OrderType)
	}
	if o.ScheduleWindowMins == nil || *o.ScheduleWindowMins != 30 {
		t.Errorf("expected scheduleWindowMins=30, got %v", o.ScheduleWindowMins)
	}
	if o.CancelDeadlineAt == nil {
		t.Error("expected cancel_deadline_at to be set")
	}
	// cancel_deadline_at = scheduledAt - 30 minutes
	expected := scheduledAt.Add(-30 * time.Minute)
	diff := o.CancelDeadlineAt.Sub(expected)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("cancel_deadline_at off by %v", diff)
	}
}

func TestUnit_CreateScheduled_TooEarly(t *testing.T) {
	svc, _ := newTestSvc()
	_, err := svc.CreateScheduled(context.Background(), CreateScheduledCommand{
		PassengerID:        "pax-early",
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(5 * time.Minute), // < 30 min
		ScheduleWindowMins: 15,
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("expected ErrBadRequest for too-early schedule, got %v", err)
	}
}

func TestUnit_CreateScheduled_ZeroWindow(t *testing.T) {
	svc, _ := newTestSvc()
	_, err := svc.CreateScheduled(context.Background(), CreateScheduledCommand{
		PassengerID:        "pax-nowin",
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(2 * time.Hour),
		ScheduleWindowMins: 0,
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("expected ErrBadRequest for zero window, got %v", err)
	}
}

func TestUnit_CreateScheduled_MissingPassenger(t *testing.T) {
	svc, _ := newTestSvc()
	_, err := svc.CreateScheduled(context.Background(), CreateScheduledCommand{
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(2 * time.Hour),
		ScheduleWindowMins: 30,
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("expected ErrBadRequest for missing passenger, got %v", err)
	}
}

func TestUnit_CreateScheduled_BlockedByActiveOrder(t *testing.T) {
	svc, store := newTestSvc()
	makeOrder(store, "pax-sched-block", StatusWaiting)

	_, err := svc.CreateScheduled(context.Background(), CreateScheduledCommand{
		PassengerID:        "pax-sched-block",
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(2 * time.Hour),
		ScheduleWindowMins: 30,
	})
	if !errors.Is(err, ErrActiveOrder) {
		t.Errorf("expected ErrActiveOrder, got %v", err)
	}
}

func TestUnit_CreateScheduled_WithPricing(t *testing.T) {
	store := newMockStore()
	pricing := &mockPricing{amount: 20000, currency: "TWD"}
	svc := NewService(store, pricing)

	id, err := svc.CreateScheduled(context.Background(), CreateScheduledCommand{
		PassengerID:        "pax-sched-price",
		Pickup:             types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:            types.Point{Lat: 25.048, Lng: 121.532},
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(2 * time.Hour),
		ScheduleWindowMins: 30,
	})
	if err != nil {
		t.Fatalf("CreateScheduled: %v", err)
	}
	o, _ := store.Get(context.Background(), id)
	if o.EstimatedFee.Amount != 20000 {
		t.Errorf("expected fee=20000, got %d", o.EstimatedFee.Amount)
	}
}

// ---------------------------------------------------------------------------
// schedule.go — ClaimScheduled
// ---------------------------------------------------------------------------

func TestUnit_ClaimScheduled_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()

	scheduledAt := time.Now().Add(2 * time.Hour)
	id, _ := svc.CreateScheduled(ctx, CreateScheduledCommand{
		PassengerID:        "pax-claim",
		RideType:           "economy",
		ScheduledAt:        scheduledAt,
		ScheduleWindowMins: 30,
	})

	if err := svc.ClaimScheduled(ctx, ClaimScheduledCommand{OrderID: id, DriverID: "drv-claim"}); err != nil {
		t.Fatalf("ClaimScheduled: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusAssigned {
		t.Errorf("expected assigned, got %s", o.Status)
	}
	if o.DriverID == nil || *o.DriverID != "drv-claim" {
		t.Errorf("expected driverID=drv-claim, got %v", o.DriverID)
	}
}

func TestUnit_ClaimScheduled_MissingIDs(t *testing.T) {
	svc, _ := newTestSvc()
	err := svc.ClaimScheduled(context.Background(), ClaimScheduledCommand{})
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

func TestUnit_ClaimScheduled_NotFound(t *testing.T) {
	svc, _ := newTestSvc()
	err := svc.ClaimScheduled(context.Background(), ClaimScheduledCommand{OrderID: "ghost", DriverID: "drv"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUnit_ClaimScheduled_WrongStatus(t *testing.T) {
	svc, store := newTestSvc()
	id := makeOrder(store, "pax-claim-wrong", StatusWaiting) // not scheduled

	err := svc.ClaimScheduled(context.Background(), ClaimScheduledCommand{OrderID: id, DriverID: "drv"})
	if !errors.Is(err, ErrInvalidState) {
		t.Errorf("expected ErrInvalidState, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// schedule.go — CancelScheduledByPassenger
// ---------------------------------------------------------------------------

func TestUnit_CancelScheduledByPassenger_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()

	id, _ := svc.CreateScheduled(ctx, CreateScheduledCommand{
		PassengerID:        "pax-pcancel",
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(2 * time.Hour),
		ScheduleWindowMins: 30,
	})

	if err := svc.CancelScheduledByPassenger(ctx, CancelScheduledCommand{OrderID: id, Reason: "changed_mind"}); err != nil {
		t.Fatalf("CancelScheduledByPassenger: %v", err)
	}
	o, _ := store.Get(ctx, id)
	if o.Status != StatusCancelled {
		t.Errorf("expected cancelled, got %s", o.Status)
	}
}

func TestUnit_CancelScheduledByPassenger_MissingID(t *testing.T) {
	svc, _ := newTestSvc()
	err := svc.CancelScheduledByPassenger(context.Background(), CancelScheduledCommand{})
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// schedule.go — CancelScheduledByDriver
// ---------------------------------------------------------------------------

func TestUnit_CancelScheduledByDriver_Success(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()

	// Create and claim an order so it's in StatusAssigned.
	id, _ := svc.CreateScheduled(ctx, CreateScheduledCommand{
		PassengerID:        "pax-dcancel",
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(2 * time.Hour),
		ScheduleWindowMins: 30,
	})
	_ = svc.ClaimScheduled(ctx, ClaimScheduledCommand{OrderID: id, DriverID: "drv-dcancel"})

	if err := svc.CancelScheduledByDriver(ctx, DriverCancelScheduledCommand{
		OrderID:  id,
		DriverID: "drv-dcancel",
	}); err != nil {
		t.Fatalf("CancelScheduledByDriver: %v", err)
	}

	o, _ := store.Get(ctx, id)
	if o.Status != StatusScheduled {
		t.Errorf("expected re-opened as scheduled, got %s", o.Status)
	}
	if o.DriverID != nil {
		t.Errorf("expected driverID to be cleared, got %s", *o.DriverID)
	}
	if o.IncentiveBonus != driverCancelBonusIncrement {
		t.Errorf("expected incentiveBonus=%d, got %d", driverCancelBonusIncrement, o.IncentiveBonus)
	}
}

func TestUnit_CancelScheduledByDriver_MissingIDs(t *testing.T) {
	svc, _ := newTestSvc()
	err := svc.CancelScheduledByDriver(context.Background(), DriverCancelScheduledCommand{})
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

func TestUnit_CancelScheduledByDriver_WrongStatus(t *testing.T) {
	svc, store := newTestSvc()
	// Order is in StatusScheduled (not Assigned).
	id, _ := svc.CreateScheduled(context.Background(), CreateScheduledCommand{
		PassengerID:        "pax-dcancel-wrong",
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(2 * time.Hour),
		ScheduleWindowMins: 30,
	})

	err := svc.CancelScheduledByDriver(context.Background(), DriverCancelScheduledCommand{
		OrderID:  id,
		DriverID: "drv-x",
	})
	_ = store
	if !errors.Is(err, ErrInvalidState) {
		t.Errorf("expected ErrInvalidState, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// schedule.go — ListScheduledByPassenger / ListAvailableScheduled
// ---------------------------------------------------------------------------

func TestUnit_ListScheduledByPassenger_Empty(t *testing.T) {
	svc, _ := newTestSvc()
	orders, err := svc.ListScheduledByPassenger(context.Background(), "pax-list-empty")
	if err != nil {
		t.Fatalf("ListScheduledByPassenger: %v", err)
	}
	if len(orders) != 0 {
		t.Errorf("expected 0 orders, got %d", len(orders))
	}
}

func TestUnit_ListScheduledByPassenger_MissingID(t *testing.T) {
	svc, _ := newTestSvc()
	_, err := svc.ListScheduledByPassenger(context.Background(), "")
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("expected ErrBadRequest, got %v", err)
	}
}

func TestUnit_ListScheduledByPassenger_ReturnsOrders(t *testing.T) {
	svc, _ := newTestSvc()
	ctx := context.Background()
	pid := types.ID("pax-list-ret")

	for i := 0; i < 3; i++ {
		_, err := svc.CreateScheduled(ctx, CreateScheduledCommand{
			PassengerID:        pid,
			RideType:           "economy",
			ScheduledAt:        time.Now().Add(time.Duration(2+i) * time.Hour),
			ScheduleWindowMins: 30,
		})
		if err != nil {
			// Once the first order is active, subsequent ones are blocked.
			break
		}
	}

	orders, err := svc.ListScheduledByPassenger(ctx, pid)
	if err != nil {
		t.Fatalf("ListScheduledByPassenger: %v", err)
	}
	if len(orders) == 0 {
		t.Error("expected at least one scheduled order")
	}
}

func TestUnit_ListAvailableScheduled_Empty(t *testing.T) {
	svc, _ := newTestSvc()
	orders, err := svc.ListAvailableScheduled(context.Background(), time.Now(), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("ListAvailableScheduled: %v", err)
	}
	if len(orders) != 0 {
		t.Errorf("expected 0 orders, got %d", len(orders))
	}
}

// ---------------------------------------------------------------------------
// Full scheduled-order happy path (unit, no DB)
// ---------------------------------------------------------------------------

func TestUnit_FullScheduledOrderFlow(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()

	scheduledAt := time.Now().Add(2 * time.Hour)
	id, err := svc.CreateScheduled(ctx, CreateScheduledCommand{
		PassengerID:        "pax-full-sched",
		Pickup:             types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:            types.Point{Lat: 25.048, Lng: 121.532},
		RideType:           "economy",
		ScheduledAt:        scheduledAt,
		ScheduleWindowMins: 30,
	})
	if err != nil {
		t.Fatalf("CreateScheduled: %v", err)
	}

	o, _ := store.Get(ctx, id)
	if o.Status != StatusScheduled {
		t.Fatalf("expected scheduled, got %s", o.Status)
	}

	if err := svc.ClaimScheduled(ctx, ClaimScheduledCommand{OrderID: id, DriverID: "drv-fs"}); err != nil {
		t.Fatalf("ClaimScheduled: %v", err)
	}
	o, _ = store.Get(ctx, id)
	if o.Status != StatusAssigned {
		t.Fatalf("expected assigned, got %s", o.Status)
	}

	if err := svc.Depart(ctx, DepartCommand{OrderID: id, DriverID: "drv-fs"}); err != nil {
		t.Fatalf("Depart: %v", err)
	}
	o, _ = store.Get(ctx, id)
	if o.Status != StatusApproaching {
		t.Fatalf("expected approaching, got %s", o.Status)
	}

	for _, step := range []struct {
		fn   func() error
		want Status
	}{
		{func() error { return svc.Arrive(ctx, ArriveCommand{OrderID: id}) }, StatusArrived},
		{func() error { return svc.Meet(ctx, MeetCommand{OrderID: id}) }, StatusDriving},
		{func() error { return svc.Complete(ctx, CompleteCommand{OrderID: id}) }, StatusPayment},
		{func() error { return svc.Pay(ctx, PayCommand{OrderID: id}) }, StatusComplete},
	} {
		if err := step.fn(); err != nil {
			t.Fatalf("step to %s: %v", step.want, err)
		}
		o, _ = store.Get(ctx, id)
		if o.Status != step.want {
			t.Errorf("expected %s, got %s", step.want, o.Status)
		}
	}
}

// ---------------------------------------------------------------------------
// Scheduled → driver cancel → re-open → new driver claims (unit)
// ---------------------------------------------------------------------------

func TestUnit_ScheduledDriverCancelReopenFlow(t *testing.T) {
	svc, store := newTestSvc()
	ctx := context.Background()

	id, _ := svc.CreateScheduled(ctx, CreateScheduledCommand{
		PassengerID:        "pax-reopen",
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(2 * time.Hour),
		ScheduleWindowMins: 30,
	})
	_ = svc.ClaimScheduled(ctx, ClaimScheduledCommand{OrderID: id, DriverID: "drv-old"})
	_ = svc.CancelScheduledByDriver(ctx, DriverCancelScheduledCommand{OrderID: id, DriverID: "drv-old"})

	o, _ := store.Get(ctx, id)
	if o.Status != StatusScheduled {
		t.Fatalf("expected re-opened as scheduled, got %s", o.Status)
	}
	if o.IncentiveBonus == 0 {
		t.Error("expected incentive bonus increase after driver cancel")
	}

	// New driver claims the re-opened order.
	if err := svc.ClaimScheduled(ctx, ClaimScheduledCommand{OrderID: id, DriverID: "drv-new"}); err != nil {
		t.Fatalf("re-claim: %v", err)
	}
	o, _ = store.Get(ctx, id)
	if o.Status != StatusAssigned {
		t.Fatalf("expected assigned after re-claim, got %s", o.Status)
	}
	if o.DriverID == nil || *o.DriverID != "drv-new" {
		t.Errorf("expected driverID=drv-new, got %v", o.DriverID)
	}
}
