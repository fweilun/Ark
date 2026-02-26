// README: Integration tests for Order store with PostgreSQL database
package order

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ark/internal/types"
)

var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	// Setup test database if available
	var err error
	testDB, err = setupIntegrationTestDB()
	if err != nil {
		fmt.Printf("Warning: Could not setup integration test database: %v\n", err)
		fmt.Println("Integration tests will be skipped. To enable them:")
		fmt.Println("1. Start PostgreSQL: docker compose up postgres-test -d")
		fmt.Println("2. Set TEST_DB_DSN environment variable")
	}

	// Run tests
	code := m.Run()

	// Cleanup
	if testDB != nil {
		testDB.Close()
	}

	os.Exit(code)
}

// setupIntegrationTestDB creates a connection pool for integration testing
func setupIntegrationTestDB() (*pgxpool.Pool, error) {
	// Check if we have a test database DSN
	testDSN := os.Getenv("TEST_DB_DSN")
	if testDSN == "" {
		// Try default local postgres setup
		testDSN = "postgres://postgres:postgres@localhost:5433/ark_test?sslmode=disable"
	}

	// Try to connect to the database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(testDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Set connection pool settings for testing
	config.MaxConns = 5
	config.MinConns = 1

	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	// Test the connection
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping test database: %w", err)
	}

	// Setup test database schema
	if err := setupTestSchema(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to setup test schema: %w", err)
	}

	return db, nil
}

// setupTestDB creates a connection pool for testing with cleanup
func setupTestDB(t *testing.T) *pgxpool.Pool {
	if testDB == nil {
		t.Skip("Integration tests skipped - no database connection available")
		return nil
	}

	// Create a test-specific schema/tables
	ctx := context.Background()
	testSchema := fmt.Sprintf("test_%s_%d",
		strings.ReplaceAll(t.Name(), "/", "_"),
		time.Now().UnixNano())

	// Create test schema
	_, err := testDB.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", testSchema))
	if err != nil {
		t.Skipf("Could not create test schema: %v", err)
		return nil
	}

	// Create test-specific connection with schema
	config := testDB.Config()
	config.ConnConfig.Database = config.ConnConfig.Database
	config.ConnConfig.RuntimeParams["search_path"] = testSchema

	testConn, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Skipf("Could not create test connection: %v", err)
		return nil
	}

	// Setup tables in test schema
	if err := setupTestTables(ctx, testConn, testSchema); err != nil {
		testConn.Close()
		t.Skipf("Could not setup test tables: %v", err)
		return nil
	}

	// Register cleanup
	t.Cleanup(func() {
		testConn.Close()
		// Drop test schema
		_, _ = testDB.Exec(context.Background(),
			fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", testSchema))
	})

	return testConn
}

// setupTestSchema creates the basic database schema if it doesn't exist
func setupTestSchema(ctx context.Context, db *pgxpool.Pool) error {
	// Check if tables exist, if not create them
	var exists bool
	err := db.QueryRow(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'orders')").Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		// Create complete schema matching production migrations
		schemaSQL := `
			CREATE TABLE IF NOT EXISTS orders (
				id TEXT PRIMARY KEY,
				passenger_id TEXT NOT NULL,
				driver_id TEXT,
				status TEXT NOT NULL,
				status_version INTEGER NOT NULL DEFAULT 1,
				pickup_lat DOUBLE PRECISION NOT NULL,
				pickup_lng DOUBLE PRECISION NOT NULL,
				dropoff_lat DOUBLE PRECISION NOT NULL,
				dropoff_lng DOUBLE PRECISION NOT NULL,
				ride_type TEXT NOT NULL,
				estimated_fee BIGINT NOT NULL,
				actual_fee BIGINT,
				order_type TEXT NOT NULL DEFAULT 'instant',
				created_at TIMESTAMP NOT NULL DEFAULT NOW(),
				matched_at TIMESTAMP,
				accepted_at TIMESTAMP,
				started_at TIMESTAMP,
				completed_at TIMESTAMP,
				cancelled_at TIMESTAMP,
				cancellation_reason TEXT,
				scheduled_at TIMESTAMP,
				schedule_window_mins INTEGER,
				cancel_deadline_at TIMESTAMP,
				incentive_bonus BIGINT DEFAULT 0,
				assigned_at TIMESTAMP
			);

			CREATE TABLE IF NOT EXISTS order_state_events (
				id BIGSERIAL PRIMARY KEY,
				order_id TEXT NOT NULL,
				from_status TEXT NOT NULL,
				to_status TEXT NOT NULL,
				actor_type TEXT NOT NULL,
				actor_id TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT NOW()
			);

			CREATE INDEX IF NOT EXISTS idx_orders_passenger ON orders (passenger_id);
			CREATE INDEX IF NOT EXISTS idx_orders_driver ON orders (driver_id);
			CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);
			CREATE INDEX IF NOT EXISTS idx_orders_created ON orders (created_at);
			CREATE INDEX IF NOT EXISTS idx_orders_scheduled ON orders (scheduled_at);
		`
		_, err = db.Exec(ctx, schemaSQL)
		return err
	}

	return nil
}

// setupTestTables creates tables in the test schema
func setupTestTables(ctx context.Context, db *pgxpool.Pool, schema string) error {
	schemaSQL := fmt.Sprintf(`
		CREATE TABLE %s.orders (
			id TEXT PRIMARY KEY,
			passenger_id TEXT NOT NULL,
			driver_id TEXT,
			status TEXT NOT NULL,
			status_version INTEGER NOT NULL DEFAULT 1,
			pickup_lat DOUBLE PRECISION NOT NULL,
			pickup_lng DOUBLE PRECISION NOT NULL,
			dropoff_lat DOUBLE PRECISION NOT NULL,
			dropoff_lng DOUBLE PRECISION NOT NULL,
			ride_type TEXT NOT NULL,
			estimated_fee BIGINT NOT NULL,
			actual_fee BIGINT,
			order_type TEXT NOT NULL DEFAULT 'instant',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			matched_at TIMESTAMP,
			accepted_at TIMESTAMP,
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			cancelled_at TIMESTAMP,
			cancellation_reason TEXT,
			scheduled_at TIMESTAMP,
			schedule_window_mins INTEGER,
			cancel_deadline_at TIMESTAMP,
			incentive_bonus BIGINT DEFAULT 0,
			assigned_at TIMESTAMP
		);

		CREATE TABLE %s.order_state_events (
			id BIGSERIAL PRIMARY KEY,
			order_id TEXT NOT NULL,
			from_status TEXT NOT NULL,
			to_status TEXT NOT NULL,
			actor_type TEXT NOT NULL,
			actor_id TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);

		CREATE INDEX idx_orders_passenger ON %s.orders (passenger_id);
		CREATE INDEX idx_orders_driver ON %s.orders (driver_id);
		CREATE INDEX idx_orders_status ON %s.orders (status);
		CREATE INDEX idx_orders_created ON %s.orders (created_at);
		CREATE INDEX idx_orders_scheduled ON %s.orders (scheduled_at);
	`, schema, schema, schema, schema, schema, schema, schema)

	_, err := db.Exec(ctx, schemaSQL)
	return err
}

func TestStore_CreateOrder_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}

	store := NewStore(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	order := &Order{
		ID:            "test-order-id",
		PassengerID:   "passenger-123",
		Status:        StatusWaiting,
		StatusVersion: 1,
		Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:      "standard",
		EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
		CreatedAt:     now,
	}

	err := store.Create(ctx, order)
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	// Verify order was created
	retrieved, err := store.Get(ctx, order.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve order: %v", err)
	}

	if retrieved.ID != order.ID {
		t.Errorf("Expected ID %s, got %s", order.ID, retrieved.ID)
	}
	if retrieved.PassengerID != order.PassengerID {
		t.Errorf("Expected PassengerID %s, got %s", order.PassengerID, retrieved.PassengerID)
	}
	if retrieved.Status != order.Status {
		t.Errorf("Expected Status %s, got %s", order.Status, retrieved.Status)
	}
	if retrieved.EstimatedFee.Amount != order.EstimatedFee.Amount {
		t.Errorf("Expected EstimatedFee %d, got %d", order.EstimatedFee.Amount, retrieved.EstimatedFee.Amount)
	}
}

func TestStore_UpdateOrder_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}

	store := NewStore(db)
	ctx := context.Background()

	// Create initial order
	now := time.Now().UTC().Truncate(time.Second)
	order := &Order{
		ID:            "test-order-update",
		PassengerID:   "passenger-456",
		Status:        StatusWaiting,
		StatusVersion: 1,
		Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:      "standard",
		EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
		CreatedAt:     now,
	}

	err := store.Create(ctx, order)
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	// Update order status
	driverID := types.ID("driver-789")

	// Update order status using UpdateStatus method
	success, err := store.UpdateStatus(ctx, order.ID, StatusWaiting, StatusApproaching, order.StatusVersion, &driverID)
	if err != nil {
		t.Fatalf("Failed to update order status: %v", err)
	}
	if !success {
		t.Fatal("UpdateStatus returned false - optimistic lock failed")
	}

	// Update our order object to reflect changes
	order.Status = StatusApproaching
	order.StatusVersion = 3 // version incremented by UpdateStatus
	order.DriverID = &driverID

	// Verify order was updated
	retrieved, err := store.Get(ctx, order.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated order: %v", err)
	}

	if retrieved.Status != StatusApproaching {
		t.Errorf("Expected Status %s, got %s", StatusApproaching, retrieved.Status)
	}
	if retrieved.StatusVersion != 2 {
		t.Errorf("Expected StatusVersion 2, got %d", retrieved.StatusVersion)
	}
	if retrieved.DriverID == nil || *retrieved.DriverID != driverID {
		t.Error("Expected DriverID to be set")
	}
	if retrieved.MatchedAt == nil {
		t.Error("Expected MatchedAt to be set")
	}
	// Note: MatchedAt is set to NOW() by database, so we can't check exact time
}

func TestStore_OptimisticLocking_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}

	store := NewStore(db)
	ctx := context.Background()

	// Create order
	now := time.Now().UTC().Truncate(time.Second)
	order := &Order{
		ID:            "test-order-lock",
		PassengerID:   "passenger-789",
		Status:        StatusWaiting,
		StatusVersion: 1,
		Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:      "standard",
		EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
		CreatedAt:     now,
	}

	err := store.Create(ctx, order)
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	// Try to update with wrong version (should fail due to optimistic locking)
	success, err := store.UpdateStatus(ctx, order.ID, StatusWaiting, StatusApproaching, 3, nil)
	if err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}
	if success {
		t.Fatal("Expected optimistic locking to prevent update, but it succeeded")
	}
}

func TestStore_GetActiveByPassenger_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}

	store := NewStore(db)
	ctx := context.Background()

	passengerID := types.ID("passenger-active")
	now := time.Now().UTC().Truncate(time.Second)

	// Create active order
	activeOrder := &Order{
		ID:            "active-order",
		PassengerID:   passengerID,
		Status:        StatusWaiting,
		StatusVersion: 1,
		Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:      "standard",
		EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
		CreatedAt:     now,
	}

	// Create completed order (should not be returned)
	completedOrder := &Order{
		ID:            "completed-order",
		PassengerID:   passengerID,
		Status:        StatusComplete,
		StatusVersion: 1,
		Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:      "standard",
		EstimatedFee:  types.Money{Currency: "USD", Amount: 1200},
		CreatedAt:     now.Add(-1 * time.Hour),
		CompletedAt:   &now,
	}

	err := store.Create(ctx, activeOrder)
	if err != nil {
		t.Fatalf("Failed to create active order: %v", err)
	}

	err = store.Create(ctx, completedOrder)
	if err != nil {
		t.Fatalf("Failed to create completed order: %v", err)
	}

	// Check if passenger has active orders
	hasActive, err := store.HasActiveByPassenger(ctx, passengerID)
	if err != nil {
		t.Fatalf("Failed to check active orders: %v", err)
	}

	// Should have active orders
	if !hasActive {
		t.Error("Expected passenger to have active orders")
	}
}

func TestStore_GetScheduledOrders_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}

	store := NewStore(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)

	// Create scheduled order (should be returned)
	pastTime := now.Add(-30 * time.Minute)
	scheduledOrder := &Order{
		ID:            "scheduled-order",
		PassengerID:   "passenger-scheduled",
		Status:        StatusScheduled,
		StatusVersion: 1,
		Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:      "standard",
		EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
		CreatedAt:     now.Add(-10 * time.Minute), // 10 minutes ago
		ScheduledAt:   &pastTime,                  // 30 minutes ago - should be returned
		OrderType:     "scheduled",
	}

	// Create recent scheduled order (should not be returned)
	futureTime := now.Add(2 * time.Hour)
	recentOrder := &Order{
		ID:            "recent-order",
		PassengerID:   "passenger-recent",
		Status:        StatusScheduled,
		StatusVersion: 1,
		Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:      "standard",
		EstimatedFee:  types.Money{Currency: "USD", Amount: 1200},
		CreatedAt:     now.Add(5 * time.Minute), // 5 minutes in future
		ScheduledAt:   &futureTime,              // 2 hours in future - should not be returned
		OrderType:     "scheduled",
	}

	err := store.CreateScheduled(ctx, scheduledOrder)
	if err != nil {
		t.Fatalf("Failed to create scheduled order: %v", err)
	}

	err = store.CreateScheduled(ctx, recentOrder)
	if err != nil {
		t.Fatalf("Failed to create recent order: %v", err)
	}

	// Get available scheduled orders within time range
	scheduled, err := store.ListAvailableScheduled(ctx, now.Add(-1*time.Hour), now)
	if err != nil {
		t.Fatalf("Failed to get scheduled orders: %v", err)
	}

	// Should only return the older scheduled order
	found := false
	for _, order := range scheduled {
		if order.ID == scheduledOrder.ID {
			found = true
		}
		if order.ID == recentOrder.ID {
			t.Error("Should not return future scheduled orders")
		}
	}

	if !found {
		t.Error("Should return past scheduled orders")
	}
}

func TestStore_ConcurrentUpdates_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}

	store := NewStore(db)
	ctx := context.Background()

	// Create order
	now := time.Now().UTC().Truncate(time.Second)
	order := &Order{
		ID:            "concurrent-test",
		PassengerID:   "passenger-concurrent",
		Status:        StatusWaiting,
		StatusVersion: 1,
		Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:      "standard",
		EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
		CreatedAt:     now,
	}

	err := store.Create(ctx, order)
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	// Simulate concurrent updates
	var successCount int
	var failCount int
	done := make(chan bool, 2)

	// First goroutine tries to assign driver and set approaching
	go func() {
		driverID := types.ID("driver-1")
		success, err := store.UpdateStatus(ctx, order.ID, StatusWaiting, StatusApproaching, 1, &driverID)
		if err == nil && success {
			successCount++
		} else {
			failCount++
		}
		done <- true
	}()

	// Second goroutine tries to cancel
	go func() {
		success, err := store.UpdateStatus(ctx, order.ID, StatusWaiting, StatusCancelled, 1, nil)
		if err == nil && success {
			successCount++
		} else {
			failCount++
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Exactly one should succeed due to optimistic locking
	if successCount != 1 || failCount != 1 {
		t.Errorf("Expected 1 success and 1 failure, got %d successes and %d failures",
			successCount, failCount)
	}
}

func TestStore_TimePrecision_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}

	store := NewStore(db)
	ctx := context.Background()

	// Use high precision timestamp
	now := time.Now().UTC()
	preciseTime := time.Date(now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second(), 123456789, time.UTC)

	order := &Order{
		ID:            "precision-test",
		PassengerID:   "passenger-precision",
		Status:        StatusWaiting,
		StatusVersion: 1,
		Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:      "standard",
		EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
		CreatedAt:     preciseTime,
	}

	err := store.Create(ctx, order)
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	// Retrieve and check precision
	retrieved, err := store.Get(ctx, order.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve order: %v", err)
	}

	// PostgreSQL typically stores microsecond precision
	// Check that we don't lose significant precision
	timeDiff := retrieved.CreatedAt.Sub(preciseTime).Abs()
	maxAllowedDiff := time.Second
	if timeDiff > maxAllowedDiff {
		t.Errorf("Time precision loss too large: %v", timeDiff)
	}
}

func TestStore_UnicodeHandling_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}

	store := NewStore(db)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	order := &Order{
		ID:            "unicode-test-訂單",
		PassengerID:   "passenger-乘客-123",
		Status:        StatusWaiting,
		StatusVersion: 1,
		Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:      "premium-豪華",
		EstimatedFee:  types.Money{Currency: "TWD", Amount: 50000}, // NT$500
		CreatedAt:     now,
	}

	err := store.Create(ctx, order)
	if err != nil {
		t.Fatalf("Failed to create order with unicode: %v", err)
	}

	// Retrieve and verify unicode preservation
	retrieved, err := store.Get(ctx, order.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve unicode order: %v", err)
	}

	if retrieved.ID != order.ID {
		t.Errorf("Unicode ID not preserved: expected %s, got %s", order.ID, retrieved.ID)
	}
	if retrieved.PassengerID != order.PassengerID {
		t.Errorf("Unicode PassengerID not preserved: expected %s, got %s",
			order.PassengerID, retrieved.PassengerID)
	}
	if retrieved.RideType != order.RideType {
		t.Errorf("Unicode RideType not preserved: expected %s, got %s",
			order.RideType, retrieved.RideType)
	}
}
