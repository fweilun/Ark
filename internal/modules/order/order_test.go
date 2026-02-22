// README: Order service tests (flow + invalid requests).
package order

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ark/internal/types"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestCanTransition verifies the state machine transition table without a database.
func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to Status
		want     bool
	}{
		// happy-path forward transitions
		{StatusWaiting, StatusApproaching, true},
		{StatusAssigned, StatusApproaching, true},
		{StatusApproaching, StatusArrived, true},
		{StatusArrived, StatusDriving, true},
		{StatusDriving, StatusPayment, true},
		{StatusPayment, StatusComplete, true},
		// cancels from every non-terminal state
		{StatusWaiting, StatusCancelled, true},
		{StatusAssigned, StatusCancelled, true},
		{StatusApproaching, StatusCancelled, true},
		{StatusArrived, StatusCancelled, true},
		{StatusDriving, StatusCancelled, true},
		// matching retries / re-match
		{StatusApproaching, StatusWaiting, true}, // driver cancel → re-match
		{StatusWaiting, StatusWaiting, true},     // self-loop retry
		// expiry / scheduled flow
		{StatusWaiting, StatusExpired, true},
		{StatusScheduled, StatusCancelled, true},
		{StatusScheduled, StatusAssigned, true},   // driver claims scheduled order
		{StatusAssigned, StatusScheduled, true},   // driver cancels → re-open
		{StatusAssigned, StatusApproaching, true}, // driver departs
		{StatusAssigned, StatusCancelled, true},   // passenger cancels assigned order
		// invalid: terminal states have no outgoing transitions
		{StatusComplete, StatusWaiting, false},
		{StatusCancelled, StatusWaiting, false},
		{StatusExpired, StatusWaiting, false},
		// invalid: skipping states
		{StatusWaiting, StatusDriving, false},
		{StatusWaiting, StatusComplete, false},
		{StatusApproaching, StatusDriving, false},
	}
	for _, tc := range cases {
		got := CanTransition(tc.from, tc.to)
		if got != tc.want {
			t.Errorf("CanTransition(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestOrderFlowHappyPath(t *testing.T) {
	svc := NewService(setupTestStore(t), nil)
	ctx := context.Background()

	orderID := mustCreateOrder(t, svc, "p_happy")
	assertStatus(t, svc, orderID, StatusWaiting)

	if err := svc.Accept(ctx, AcceptCommand{OrderID: orderID, DriverID: "d1"}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	assertStatus(t, svc, orderID, StatusApproaching)

	if err := svc.Arrive(ctx, ArriveCommand{OrderID: orderID}); err != nil {
		t.Fatalf("arrive: %v", err)
	}
	assertStatus(t, svc, orderID, StatusArrived)

	if err := svc.Meet(ctx, MeetCommand{OrderID: orderID}); err != nil {
		t.Fatalf("meet: %v", err)
	}
	assertStatus(t, svc, orderID, StatusDriving)

	if err := svc.Complete(ctx, CompleteCommand{OrderID: orderID}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	assertStatus(t, svc, orderID, StatusPayment)

	if err := svc.Pay(ctx, PayCommand{OrderID: orderID}); err != nil {
		t.Fatalf("pay: %v", err)
	}
	assertStatus(t, svc, orderID, StatusComplete)
}

func TestOrderFlowAcceptSameTime(t *testing.T) {
	svc := NewService(setupTestStore(t), nil)
	ctx := context.Background()

	orderID := mustCreateOrder(t, svc, "p_accept_same_time")

	driverIDs := []types.ID{"d1", "d2", "d3"}
	errs := make(chan error, len(driverIDs))
	start := make(chan struct{})
	var wg sync.WaitGroup

	for _, driverID := range driverIDs {
		wg.Add(1)
		go func(did types.ID) {
			defer wg.Done()
			<-start
			errs <- svc.Accept(ctx, AcceptCommand{OrderID: orderID, DriverID: did})
		}(driverID)
	}

	close(start)
	wg.Wait()
	close(errs)

	success := 0
	for err := range errs {
		if err == nil {
			success++
			continue
		}
		if err != ErrConflict && err != ErrInvalidState {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if success != 1 {
		t.Fatalf("expected exactly 1 success, got %d", success)
	}

	assertStatus(t, svc, orderID, StatusApproaching)
}

func TestOrderFlowCancelWaiting(t *testing.T) {
	svc := NewService(setupTestStore(t), nil)
	ctx := context.Background()

	orderID := mustCreateOrder(t, svc, "p_cancel_waiting")
	if err := svc.Cancel(ctx, CancelCommand{OrderID: orderID, ActorType: "passenger", Reason: "user_cancel"}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	assertStatus(t, svc, orderID, StatusCancelled)

	if err := svc.Accept(ctx, AcceptCommand{OrderID: orderID, DriverID: "d1"}); err != ErrInvalidState {
		t.Fatalf("accept after cancel: expected ErrInvalidState, got %v", err)
	}
}

func TestOrderFlowCancelApproaching(t *testing.T) {
	svc := NewService(setupTestStore(t), nil)
	ctx := context.Background()

	orderID := mustCreateOrder(t, svc, "p_cancel_approaching")
	if err := svc.Accept(ctx, AcceptCommand{OrderID: orderID, DriverID: "d1"}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if err := svc.Cancel(ctx, CancelCommand{OrderID: orderID, ActorType: "driver", Reason: "driver_cancel"}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	assertStatus(t, svc, orderID, StatusCancelled)
}

func TestOrderInvalidTransitions(t *testing.T) {
	svc := NewService(setupTestStore(t), nil)
	ctx := context.Background()

	orderID := mustCreateOrder(t, svc, "p_invalid")

	if err := svc.Arrive(ctx, ArriveCommand{OrderID: orderID}); err != ErrInvalidState {
		t.Fatalf("arrive before accept: expected ErrInvalidState, got %v", err)
	}
	if err := svc.Meet(ctx, MeetCommand{OrderID: orderID}); err != ErrInvalidState {
		t.Fatalf("meet before arrive: expected ErrInvalidState, got %v", err)
	}
	if err := svc.Complete(ctx, CompleteCommand{OrderID: orderID}); err != ErrInvalidState {
		t.Fatalf("complete before driving: expected ErrInvalidState, got %v", err)
	}
	if err := svc.Pay(ctx, PayCommand{OrderID: orderID}); err != ErrInvalidState {
		t.Fatalf("pay before payment: expected ErrInvalidState, got %v", err)
	}

	if err := svc.Accept(ctx, AcceptCommand{OrderID: orderID, DriverID: "d1"}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if err := svc.Meet(ctx, MeetCommand{OrderID: orderID}); err != ErrInvalidState {
		t.Fatalf("meet before arrive (after accept): expected ErrInvalidState, got %v", err)
	}
}

func TestConcurrentAcceptVsCancel(t *testing.T) {
	ctx := context.Background()
	store := setupTestStore(t)
	svc := NewService(store, nil)

	orderID, err := svc.Create(ctx, CreateCommand{
		PassengerID: "p_accept_cancel",
		Pickup:      types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:     types.Point{Lat: 25.0478, Lng: 121.5318},
		RideType:    "economy",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- svc.Accept(ctx, AcceptCommand{OrderID: orderID, DriverID: "d1"})
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- svc.Cancel(ctx, CancelCommand{OrderID: orderID, ActorType: "passenger", Reason: "user_cancel"})
	}()

	wg.Wait()
	close(errs)

	success := 0
	for err := range errs {
		if err == nil {
			success++
			continue
		}
		if err != ErrConflict && err != ErrInvalidState {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if success < 1 || success > 2 {
		t.Fatalf("expected 1 or 2 successes, got %d", success)
	}

	o, err := svc.Get(ctx, orderID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if success == 2 && o.Status != StatusCancelled {
		t.Fatalf("expected cancelled after accept+cancel, got %s", o.Status)
	}
	if success == 1 && o.Status != StatusApproaching && o.Status != StatusCancelled {
		t.Fatalf("unexpected final status: %s", o.Status)
	}
}

func TestConcurrentAcceptSameOrder(t *testing.T) {
	ctx := context.Background()
	store := setupTestStore(t)
	svc := NewService(store, nil)

	orderID, err := svc.Create(ctx, CreateCommand{
		PassengerID: "p_multi_accept",
		Pickup:      types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:     types.Point{Lat: 25.0478, Lng: 121.5318},
		RideType:    "economy",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	const attempts = 8
	var wg sync.WaitGroup
	errs := make(chan error, attempts)

	for i := 0; i < attempts; i++ {
		driverID := types.ID(fmt.Sprintf("d%d", i))
		wg.Add(1)
		go func(did types.ID) {
			defer wg.Done()
			errs <- svc.Accept(ctx, AcceptCommand{OrderID: orderID, DriverID: did})
		}(driverID)
	}

	wg.Wait()
	close(errs)

	success := 0
	for err := range errs {
		if err == nil {
			success++
			continue
		}
		if err != ErrConflict && err != ErrInvalidState {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("expected exactly 1 success, got %d", success)
	}

	o, err := svc.Get(ctx, orderID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if o.Status != StatusApproaching {
		t.Fatalf("unexpected final status: %s", o.Status)
	}
	if o.DriverID == nil || *o.DriverID == "" {
		t.Fatalf("expected driver_id to be set")
	}
}

func TestRejectOrderAtAnyTime(t *testing.T) {
	ctx := context.Background()
	store := setupTestStore(t)
	svc := NewService(store, nil)

	t.Run("passenger_cancel_while_waiting", func(t *testing.T) {
		orderID, err := svc.Create(ctx, CreateCommand{
			PassengerID: "p_cancel_waiting",
			Pickup:      types.Point{Lat: 25.033, Lng: 121.565},
			Dropoff:     types.Point{Lat: 25.0478, Lng: 121.5318},
			RideType:    "economy",
		})
		if err != nil {
			t.Fatalf("create order: %v", err)
		}

		if err := svc.Cancel(ctx, CancelCommand{OrderID: orderID, ActorType: "passenger", Reason: "user_cancel"}); err != nil {
			t.Fatalf("cancel order: %v", err)
		}

		o, err := svc.Get(ctx, orderID)
		if err != nil {
			t.Fatalf("get order: %v", err)
		}
		if o.Status != StatusCancelled {
			t.Fatalf("expected status cancelled, got %s", o.Status)
		}
	})

	t.Run("driver_cancel_while_approaching", func(t *testing.T) {
		orderID, err := svc.Create(ctx, CreateCommand{
			PassengerID: "p_cancel_approaching",
			Pickup:      types.Point{Lat: 25.033, Lng: 121.565},
			Dropoff:     types.Point{Lat: 25.0478, Lng: 121.5318},
			RideType:    "economy",
		})
		if err != nil {
			t.Fatalf("create order: %v", err)
		}

		if err := svc.Accept(ctx, AcceptCommand{OrderID: orderID, DriverID: "d_cancel"}); err != nil {
			t.Fatalf("accept order: %v", err)
		}

		if err := svc.Cancel(ctx, CancelCommand{OrderID: orderID, ActorType: "driver", Reason: "driver_cancel"}); err != nil {
			t.Fatalf("cancel order: %v", err)
		}

		o, err := svc.Get(ctx, orderID)
		if err != nil {
			t.Fatalf("get order: %v", err)
		}
		if o.Status != StatusCancelled {
			t.Fatalf("expected status cancelled, got %s", o.Status)
		}
	})
}

func TestOrderFlowRematch(t *testing.T) {
	svc := NewService(setupTestStore(t), nil)
	ctx := context.Background()

	// Driver accepts, then cancels while approaching → order returns to Waiting.
	orderID := mustCreateOrder(t, svc, "p_rematch")
	if err := svc.Accept(ctx, AcceptCommand{OrderID: orderID, DriverID: "d_cancel"}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	assertStatus(t, svc, orderID, StatusApproaching)

	if err := svc.Rematch(ctx, RematchCommand{OrderID: orderID}); err != nil {
		t.Fatalf("rematch: %v", err)
	}
	assertStatus(t, svc, orderID, StatusWaiting)

	// A new driver can now accept the re-queued order.
	if err := svc.Accept(ctx, AcceptCommand{OrderID: orderID, DriverID: "d_new"}); err != nil {
		t.Fatalf("accept after rematch: %v", err)
	}
	assertStatus(t, svc, orderID, StatusApproaching)
}

func TestScheduledOrderFlowHappyPath(t *testing.T) {
	svc := NewService(setupTestStore(t), nil)
	ctx := context.Background()

	orderID := mustCreateScheduledOrder(t, svc, "p_sched_happy")
	assertStatus(t, svc, orderID, StatusScheduled)

	// Check fields were persisted.
	o, err := svc.Get(ctx, orderID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if o.OrderType != "scheduled" {
		t.Fatalf("expected order_type=scheduled, got %s", o.OrderType)
	}
	if o.ScheduledAt == nil {
		t.Fatal("expected scheduled_at to be set")
	}
	if o.CancelDeadlineAt == nil {
		t.Fatal("expected cancel_deadline_at to be set")
	}
	if o.ScheduleWindowMins == nil || *o.ScheduleWindowMins != 30 {
		t.Fatalf("expected schedule_window_mins=30, got %v", o.ScheduleWindowMins)
	}

	// Driver claims the order.
	if err := svc.ClaimScheduled(ctx, ClaimScheduledCommand{OrderID: orderID, DriverID: "d_sched_1"}); err != nil {
		t.Fatalf("claim: %v", err)
	}
	assertStatus(t, svc, orderID, StatusAssigned)

	// Driver departs (Assigned → Approaching).
	if err := svc.Depart(ctx, DepartCommand{OrderID: orderID, DriverID: "d_sched_1"}); err != nil {
		t.Fatalf("depart: %v", err)
	}
	assertStatus(t, svc, orderID, StatusApproaching)
}

func TestScheduledOrderValidation(t *testing.T) {
	svc := NewService(setupTestStore(t), nil)
	ctx := context.Background()

	// scheduled_at must be at least 30 minutes in the future.
	_, err := svc.CreateScheduled(ctx, CreateScheduledCommand{
		PassengerID:        "p_sched_val",
		Pickup:             types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:            types.Point{Lat: 25.0478, Lng: 121.5318},
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(10 * time.Minute), // too soon
		ScheduleWindowMins: 30,
	})
	if err != ErrBadRequest {
		t.Fatalf("expected ErrBadRequest for near-future scheduled_at, got %v", err)
	}

	// schedule_window_mins must be positive.
	_, err = svc.CreateScheduled(ctx, CreateScheduledCommand{
		PassengerID:        "p_sched_val2",
		Pickup:             types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:            types.Point{Lat: 25.0478, Lng: 121.5318},
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(2 * time.Hour),
		ScheduleWindowMins: 0,
	})
	if err != ErrBadRequest {
		t.Fatalf("expected ErrBadRequest for zero schedule_window_mins, got %v", err)
	}

	// Missing ride_type.
	_, err = svc.CreateScheduled(ctx, CreateScheduledCommand{
		PassengerID:        "p_sched_val3",
		Pickup:             types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:            types.Point{Lat: 25.0478, Lng: 121.5318},
		ScheduledAt:        time.Now().Add(2 * time.Hour),
		ScheduleWindowMins: 30,
	})
	if err != ErrBadRequest {
		t.Fatalf("expected ErrBadRequest for missing ride_type, got %v", err)
	}
}

func TestScheduledOrderActiveConflict(t *testing.T) {
	svc := NewService(setupTestStore(t), nil)
	ctx := context.Background()

	// Creating a scheduled order blocks a second one for the same passenger.
	passengerID := types.ID("p_sched_conflict")
	if _, err := svc.CreateScheduled(ctx, CreateScheduledCommand{
		PassengerID:        passengerID,
		Pickup:             types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:            types.Point{Lat: 25.0478, Lng: 121.5318},
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(2 * time.Hour),
		ScheduleWindowMins: 30,
	}); err != nil {
		t.Fatalf("first scheduled order: %v", err)
	}
	_, err := svc.CreateScheduled(ctx, CreateScheduledCommand{
		PassengerID:        passengerID,
		Pickup:             types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:            types.Point{Lat: 25.0478, Lng: 121.5318},
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(3 * time.Hour),
		ScheduleWindowMins: 30,
	})
	if err != ErrActiveOrder {
		t.Fatalf("expected ErrActiveOrder for duplicate scheduled order, got %v", err)
	}

	// Instant order is also blocked while scheduled order is active.
	_, err = svc.Create(ctx, CreateCommand{
		PassengerID: passengerID,
		Pickup:      types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:     types.Point{Lat: 25.0478, Lng: 121.5318},
		RideType:    "economy",
	})
	if err != ErrActiveOrder {
		t.Fatalf("expected ErrActiveOrder for instant order while scheduled is active, got %v", err)
	}
}

func TestScheduledOrderDriverCancel(t *testing.T) {
	svc := NewService(setupTestStore(t), nil)
	ctx := context.Background()

	orderID := mustCreateScheduledOrder(t, svc, "p_sched_dcancel")
	assertStatus(t, svc, orderID, StatusScheduled)

	// Driver claims.
	if err := svc.ClaimScheduled(ctx, ClaimScheduledCommand{OrderID: orderID, DriverID: "d_sched_cancel"}); err != nil {
		t.Fatalf("claim: %v", err)
	}
	assertStatus(t, svc, orderID, StatusAssigned)

	// Driver cancels → order re-opens as 'scheduled'.
	if err := svc.CancelScheduledByDriver(ctx, DriverCancelScheduledCommand{
		OrderID:  orderID,
		DriverID: "d_sched_cancel",
	}); err != nil {
		t.Fatalf("driver cancel: %v", err)
	}
	assertStatus(t, svc, orderID, StatusScheduled)

	o, err := svc.Get(ctx, orderID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if o.DriverID != nil {
		t.Fatalf("expected driver_id to be cleared after re-open, got %s", *o.DriverID)
	}
	if o.IncentiveBonus == 0 {
		t.Fatal("expected incentive_bonus to be increased after driver cancel")
	}

	// A new driver can now claim the re-opened order.
	if err := svc.ClaimScheduled(ctx, ClaimScheduledCommand{OrderID: orderID, DriverID: "d_sched_new"}); err != nil {
		t.Fatalf("claim by new driver: %v", err)
	}
	assertStatus(t, svc, orderID, StatusAssigned)
}

func TestScheduledOrderPassengerCancel(t *testing.T) {
	svc := NewService(setupTestStore(t), nil)
	ctx := context.Background()

	orderID := mustCreateScheduledOrder(t, svc, "p_sched_pcancel")
	assertStatus(t, svc, orderID, StatusScheduled)

	if err := svc.CancelScheduledByPassenger(ctx, CancelScheduledCommand{
		OrderID: orderID,
		Reason:  "user_cancel",
	}); err != nil {
		t.Fatalf("passenger cancel: %v", err)
	}
	assertStatus(t, svc, orderID, StatusCancelled)
}

func TestConcurrentClaimScheduled(t *testing.T) {
	ctx := context.Background()
	store := setupTestStore(t)
	svc := NewService(store, nil)

	orderID := mustCreateScheduledOrder(t, svc, "p_sched_concurrent")

	const attempts = 6
	var wg sync.WaitGroup
	errs := make(chan error, attempts)

	for i := 0; i < attempts; i++ {
		driverID := types.ID(fmt.Sprintf("d_sched_%d", i))
		wg.Add(1)
		go func(did types.ID) {
			defer wg.Done()
			errs <- svc.ClaimScheduled(ctx, ClaimScheduledCommand{OrderID: orderID, DriverID: did})
		}(driverID)
	}

	wg.Wait()
	close(errs)

	success := 0
	for err := range errs {
		if err == nil {
			success++
			continue
		}
		if err != ErrConflict && err != ErrInvalidState {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("expected exactly 1 successful claim, got %d", success)
	}

	o, err := svc.Get(ctx, orderID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if o.Status != StatusAssigned {
		t.Fatalf("expected status assigned, got %s", o.Status)
	}
	if o.DriverID == nil {
		t.Fatal("expected driver_id to be set after claim")
	}
}

func mustCreateOrder(t *testing.T, svc *Service, passengerID types.ID) types.ID {
	t.Helper()
	id, err := svc.Create(context.Background(), CreateCommand{
		PassengerID: passengerID,
		Pickup:      types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:     types.Point{Lat: 25.0478, Lng: 121.5318},
		RideType:    "economy",
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	return id
}

func mustCreateScheduledOrder(t *testing.T, svc *Service, passengerID types.ID) types.ID {
	t.Helper()
	id, err := svc.CreateScheduled(context.Background(), CreateScheduledCommand{
		PassengerID:        passengerID,
		Pickup:             types.Point{Lat: 25.033, Lng: 121.565},
		Dropoff:            types.Point{Lat: 25.0478, Lng: 121.5318},
		RideType:           "economy",
		ScheduledAt:        time.Now().Add(2 * time.Hour),
		ScheduleWindowMins: 30,
	})
	if err != nil {
		t.Fatalf("create scheduled order: %v", err)
	}
	return id
}

func assertStatus(t *testing.T, svc *Service, orderID types.ID, want Status) {
	t.Helper()
	o, err := svc.Get(context.Background(), orderID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if o.Status != want {
		t.Fatalf("expected status %s, got %s", want, o.Status)
	}
}

func setupTestStore(t *testing.T) *Store {
	t.Helper()

	dsn := os.Getenv("ARK_TEST_DSN")
	if dsn == "" {
		t.Skip("ARK_TEST_DSN not set; skipping DB-backed race tests")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := applyMigration(ctx, db); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	if _, err := db.Exec(ctx, "TRUNCATE TABLE order_state_events, orders"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	return NewStore(db)
}

func applyMigration(ctx context.Context, db *pgxpool.Pool) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	for _, name := range []string{"0001_init.sql", "0002_schedule.sql"} {
		path := filepath.Join(root, "migrations", name)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		cleaned := stripSQLComments(string(content))
		for _, stmt := range splitSQL(cleaned) {
			if _, err := db.Exec(ctx, stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}

func stripSQLComments(input string) string {
	var b strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		b.WriteString(scanner.Text())
		b.WriteString("\n")
	}
	return b.String()
}

func splitSQL(input string) []string {
	parts := strings.Split(input, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		stmt := strings.TrimSpace(p)
		if stmt == "" {
			continue
		}
		out = append(out, stmt)
	}
	return out
}
