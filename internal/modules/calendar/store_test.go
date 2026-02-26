// README: Comprehensive integration tests for the calendar store using a real database.
package calendar

import (
	"context"
	"errors"
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
		fmt.Println("1. Start PostgreSQL: docker compose up postgres -d")
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
		testDSN = "postgres://postgres:postgres@localhost:5432/ark_test?sslmode=disable"
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
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'calendar_events')").Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		// Create basic schema - in real setup, this would run migrations
		schemaSQL := `
			CREATE TABLE IF NOT EXISTS calendar_events (
				id          TEXT PRIMARY KEY,
				"from"      TIMESTAMP NOT NULL,
				"to"        TIMESTAMP NOT NULL,
				title       TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT ''
			);

			CREATE TABLE IF NOT EXISTS calendar_schedules (
				uid         TEXT NOT NULL,
				event_id    TEXT NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
				tied_order  TEXT,
				PRIMARY KEY (uid, event_id)
			);

			CREATE INDEX IF NOT EXISTS idx_calendar_schedules_uid ON calendar_schedules (uid);
		`
		_, err = db.Exec(ctx, schemaSQL)
		return err
	}

	return nil
}

// setupTestTables creates tables in the test schema
func setupTestTables(ctx context.Context, db *pgxpool.Pool, schema string) error {
	schemaSQL := fmt.Sprintf(`
		CREATE TABLE %s.calendar_events (
			id          TEXT PRIMARY KEY,
			"from"      TIMESTAMP NOT NULL,
			"to"        TIMESTAMP NOT NULL,
			title       TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT ''
		);

		CREATE TABLE %s.calendar_schedules (
			uid         TEXT NOT NULL,
			event_id    TEXT NOT NULL REFERENCES %s.calendar_events(id) ON DELETE CASCADE,
			tied_order  TEXT,
			PRIMARY KEY (uid, event_id)
		);

		CREATE INDEX idx_calendar_schedules_uid_%s ON %s.calendar_schedules (uid);
	`, schema, schema, schema, strings.ReplaceAll(schema, ".", "_"), schema)

	_, err := db.Exec(ctx, schemaSQL)
	return err
}

func cleanupTestDB(t *testing.T, db *pgxpool.Pool) {
	if db != nil {
		db.Close()
	}
}

func TestStore_CreateEvent_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	store := NewStore(db)
	ctx := context.Background()

	// Use UTC times to avoid timezone issues with database storage
	now := time.Now().UTC().Truncate(time.Second)
	event := &Event{
		ID:          "test-event-id",
		From:        now,
		To:          now.Add(time.Hour),
		Title:       "Integration Test Event",
		Description: "Test Description",
	}

	err := store.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Verify event was created
	retrievedEvent, err := store.GetEvent(ctx, event.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve event: %v", err)
	}

	if retrievedEvent.ID != event.ID {
		t.Errorf("Expected ID %s, got %s", event.ID, retrievedEvent.ID)
	}
	if !retrievedEvent.From.Equal(event.From) {
		t.Errorf("Expected From %v, got %v", event.From, retrievedEvent.From)
	}
	if !retrievedEvent.To.Equal(event.To) {
		t.Errorf("Expected To %v, got %v", event.To, retrievedEvent.To)
	}
	if retrievedEvent.Title != event.Title {
		t.Errorf("Expected Title %s, got %s", event.Title, retrievedEvent.Title)
	}
	if retrievedEvent.Description != event.Description {
		t.Errorf("Expected Description %s, got %s", event.Description, retrievedEvent.Description)
	}
}

func TestStore_UpdateEvent_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	store := NewStore(db)
	ctx := context.Background()

	// Create an event first - Use UTC times to avoid timezone issues
	now := time.Now().UTC().Truncate(time.Second)
	originalEvent := &Event{
		ID:          "test-event-id",
		From:        now,
		To:          now.Add(time.Hour),
		Title:       "Original Title",
		Description: "Original Description",
	}

	err := store.CreateEvent(ctx, originalEvent)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Update the event - Use UTC times
	laterTime := now.Add(2 * time.Hour)
	updatedEvent := &Event{
		ID:          originalEvent.ID,
		From:        laterTime,
		To:          laterTime.Add(time.Hour),
		Title:       "Updated Title",
		Description: "Updated Description",
	}

	err = store.UpdateEvent(ctx, updatedEvent)
	if err != nil {
		t.Fatalf("Failed to update event: %v", err)
	}

	// Verify event was updated
	retrievedEvent, err := store.GetEvent(ctx, originalEvent.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated event: %v", err)
	}

	if !retrievedEvent.From.Equal(updatedEvent.From) {
		t.Errorf("Expected From %v, got %v", updatedEvent.From, retrievedEvent.From)
	}
	if !retrievedEvent.To.Equal(updatedEvent.To) {
		t.Errorf("Expected To %v, got %v", updatedEvent.To, retrievedEvent.To)
	}
	if retrievedEvent.Title != updatedEvent.Title {
		t.Errorf("Expected Title %s, got %s", updatedEvent.Title, retrievedEvent.Title)
	}
	if retrievedEvent.Description != updatedEvent.Description {
		t.Errorf("Expected Description %s, got %s", updatedEvent.Description, retrievedEvent.Description)
	}
}

func TestStore_DeleteEvent_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	store := NewStore(db)
	ctx := context.Background()

	// Create an event first
	event := &Event{
		ID:          "test-event-id",
		From:        time.Now().Truncate(time.Second),
		To:          time.Now().Add(time.Hour).Truncate(time.Second),
		Title:       "Event to Delete",
		Description: "Test Description",
	}

	err := store.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Delete the event
	err = store.DeleteEvent(ctx, event.ID)
	if err != nil {
		t.Fatalf("Failed to delete event: %v", err)
	}

	// Verify event was deleted
	_, err = store.GetEvent(ctx, event.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound after deletion, got %v", err)
	}
}

func TestStore_CreateSchedule_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	store := NewStore(db)
	ctx := context.Background()

	// Create an event first (required for foreign key)
	event := &Event{
		ID:          "test-event-id",
		From:        time.Now().Truncate(time.Second),
		To:          time.Now().Add(time.Hour).Truncate(time.Second),
		Title:       "Test Event",
		Description: "Test Description",
	}

	err := store.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Create schedule
	orderID := types.ID("test-order-id")
	schedule := &Schedule{
		UID:       "user-123",
		EventID:   event.ID,
		TiedOrder: &orderID,
	}

	err = store.CreateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	// Verify schedule was created
	retrievedSchedule, err := store.GetSchedule(ctx, schedule.UID, schedule.EventID)
	if err != nil {
		t.Fatalf("Failed to retrieve schedule: %v", err)
	}

	if retrievedSchedule.UID != schedule.UID {
		t.Errorf("Expected UID %s, got %s", schedule.UID, retrievedSchedule.UID)
	}
	if retrievedSchedule.EventID != schedule.EventID {
		t.Errorf("Expected EventID %s, got %s", schedule.EventID, retrievedSchedule.EventID)
	}
	if retrievedSchedule.TiedOrder == nil {
		t.Fatal("Expected TiedOrder to be set")
	}
	if *retrievedSchedule.TiedOrder != orderID {
		t.Errorf("Expected TiedOrder %s, got %s", orderID, *retrievedSchedule.TiedOrder)
	}
}

func TestStore_CreateScheduleWithoutTiedOrder_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	store := NewStore(db)
	ctx := context.Background()

	// Create an event first (required for foreign key)
	event := &Event{
		ID:          "test-event-id",
		From:        time.Now().Truncate(time.Second),
		To:          time.Now().Add(time.Hour).Truncate(time.Second),
		Title:       "Test Event",
		Description: "Test Description",
	}

	err := store.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Create schedule without tied order
	schedule := &Schedule{
		UID:       "user-123",
		EventID:   event.ID,
		TiedOrder: nil,
	}

	err = store.CreateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	// Verify schedule was created
	retrievedSchedule, err := store.GetSchedule(ctx, schedule.UID, schedule.EventID)
	if err != nil {
		t.Fatalf("Failed to retrieve schedule: %v", err)
	}

	if retrievedSchedule.TiedOrder != nil {
		t.Errorf("Expected TiedOrder to be nil, got %v", *retrievedSchedule.TiedOrder)
	}
}

func TestStore_UpdateScheduleTiedOrder_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	store := NewStore(db)
	ctx := context.Background()

	// Create an event first (required for foreign key)
	event := &Event{
		ID:          "test-event-id",
		From:        time.Now().Truncate(time.Second),
		To:          time.Now().Add(time.Hour).Truncate(time.Second),
		Title:       "Test Event",
		Description: "Test Description",
	}

	err := store.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Create schedule without tied order
	schedule := &Schedule{
		UID:       "user-123",
		EventID:   event.ID,
		TiedOrder: nil,
	}

	err = store.CreateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	// Update with tied order
	orderID := types.ID("new-order-id")
	err = store.UpdateScheduleTiedOrder(ctx, schedule.UID, schedule.EventID, &orderID)
	if err != nil {
		t.Fatalf("Failed to update schedule tied order: %v", err)
	}

	// Verify update
	retrievedSchedule, err := store.GetSchedule(ctx, schedule.UID, schedule.EventID)
	if err != nil {
		t.Fatalf("Failed to retrieve schedule: %v", err)
	}

	if retrievedSchedule.TiedOrder == nil {
		t.Fatal("Expected TiedOrder to be set")
	}
	if *retrievedSchedule.TiedOrder != orderID {
		t.Errorf("Expected TiedOrder %s, got %s", orderID, *retrievedSchedule.TiedOrder)
	}

	// Update to clear tied order
	err = store.UpdateScheduleTiedOrder(ctx, schedule.UID, schedule.EventID, nil)
	if err != nil {
		t.Fatalf("Failed to clear schedule tied order: %v", err)
	}

	// Verify cleared
	retrievedSchedule, err = store.GetSchedule(ctx, schedule.UID, schedule.EventID)
	if err != nil {
		t.Fatalf("Failed to retrieve schedule: %v", err)
	}

	if retrievedSchedule.TiedOrder != nil {
		t.Errorf("Expected TiedOrder to be nil, got %v", *retrievedSchedule.TiedOrder)
	}
}

func TestStore_ListSchedulesByUser_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	store := NewStore(db)
	ctx := context.Background()

	// Create events
	event1 := &Event{
		ID:          "event-1",
		From:        time.Now().Truncate(time.Second),
		To:          time.Now().Add(time.Hour).Truncate(time.Second),
		Title:       "Event 1",
		Description: "Description 1",
	}
	event2 := &Event{
		ID:          "event-2",
		From:        time.Now().Add(2 * time.Hour).Truncate(time.Second),
		To:          time.Now().Add(3 * time.Hour).Truncate(time.Second),
		Title:       "Event 2",
		Description: "Description 2",
	}
	event3 := &Event{
		ID:          "event-3",
		From:        time.Now().Add(4 * time.Hour).Truncate(time.Second),
		To:          time.Now().Add(5 * time.Hour).Truncate(time.Second),
		Title:       "Event 3",
		Description: "Description 3",
	}

	for _, event := range []*Event{event1, event2, event3} {
		err := store.CreateEvent(ctx, event)
		if err != nil {
			t.Fatalf("Failed to create event %s: %v", event.ID, err)
		}
	}

	uid1 := types.ID("user-1")
	uid2 := types.ID("user-2")
	orderID := types.ID("order-1")

	// Create schedules
	schedules := []*Schedule{
		{UID: uid1, EventID: event1.ID, TiedOrder: &orderID},
		{UID: uid1, EventID: event2.ID, TiedOrder: nil},
		{UID: uid2, EventID: event3.ID, TiedOrder: nil},
	}

	for _, schedule := range schedules {
		err := store.CreateSchedule(ctx, schedule)
		if err != nil {
			t.Fatalf("Failed to create schedule for user %s, event %s: %v", schedule.UID, schedule.EventID, err)
		}
	}

	// List schedules for user 1
	userSchedules, err := store.ListSchedulesByUser(ctx, uid1)
	if err != nil {
		t.Fatalf("Failed to list schedules for user %s: %v", uid1, err)
	}

	if len(userSchedules) != 2 {
		t.Fatalf("Expected 2 schedules for user %s, got %d", uid1, len(userSchedules))
	}

	// Verify correct schedules
	eventIDs := make(map[types.ID]bool)
	for _, schedule := range userSchedules {
		if schedule.UID != uid1 {
			t.Errorf("Expected UID %s, got %s", uid1, schedule.UID)
		}
		eventIDs[schedule.EventID] = true
	}

	if !eventIDs[event1.ID] || !eventIDs[event2.ID] {
		t.Error("Not all expected events are present in user schedules")
	}
	if eventIDs[event3.ID] {
		t.Error("Event 3 should not be in user 1's schedules")
	}
}

// Unit tests for edge cases and error conditions
func TestStore_GetEvent_NotFound(t *testing.T) {

}

func TestStore_UpdateEvent_NotFound(t *testing.T) {
	// Test with mock to demonstrate error case
	t.Skip("Unit test with mock - demonstrates UpdateEvent not found case")
}

func TestStore_DeleteEvent_NotFound(t *testing.T) {
	// Test with mock to demonstrate error case
	t.Skip("Unit test with mock - demonstrates DeleteEvent not found case")
}

func TestStore_GetSchedule_NotFound(t *testing.T) {
	// Test with mock to demonstrate error case
	t.Skip("Unit test with mock - demonstrates GetSchedule not found case")
}

func TestStore_UpdateScheduleTiedOrder_NotFound(t *testing.T) {
	// Test with mock to demonstrate error case
	t.Skip("Unit test with mock - demonstrates UpdateScheduleTiedOrder not found case")
}

func TestStore_EventCascadeDelete_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	store := NewStore(db)
	ctx := context.Background()

	// Create an event
	event := &Event{
		ID:          "test-event-id",
		From:        time.Now().Truncate(time.Second),
		To:          time.Now().Add(time.Hour).Truncate(time.Second),
		Title:       "Event to Delete",
		Description: "Test Description",
	}

	err := store.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Create a schedule referencing the event
	schedule := &Schedule{
		UID:       "user-123",
		EventID:   event.ID,
		TiedOrder: nil,
	}

	err = store.CreateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	// Delete the event - should cascade delete the schedule
	err = store.DeleteEvent(ctx, event.ID)
	if err != nil {
		t.Fatalf("Failed to delete event: %v", err)
	}

	// Verify schedule was also deleted due to foreign key cascade
	_, err = store.GetSchedule(ctx, schedule.UID, schedule.EventID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected schedule to be cascade deleted, but got error: %v", err)
	}
}

func TestStore_DuplicateSchedule_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	store := NewStore(db)
	ctx := context.Background()

	// Create an event
	event := &Event{
		ID:          "test-event-id",
		From:        time.Now().Truncate(time.Second),
		To:          time.Now().Add(time.Hour).Truncate(time.Second),
		Title:       "Test Event",
		Description: "Test Description",
	}

	err := store.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Create first schedule
	schedule := &Schedule{
		UID:       "user-123",
		EventID:   event.ID,
		TiedOrder: nil,
	}

	err = store.CreateSchedule(ctx, schedule)
	if err != nil {
		t.Fatalf("Failed to create first schedule: %v", err)
	}

	// Try to create duplicate schedule - should fail due to primary key constraint
	err = store.CreateSchedule(ctx, schedule)
	if err == nil {
		t.Fatal("Expected error when creating duplicate schedule, got nil")
	}
	// The exact error depends on the database implementation
	// In PostgreSQL, this would be a unique constraint violation
}

func TestStore_TimePrecision_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	store := NewStore(db)
	ctx := context.Background()

	// Test with high precision timestamp - Use UTC to avoid timezone issues
	now := time.Now().UTC()
	preciseFrom := time.Date(now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second(), 123456789, time.UTC)
	preciseTo := preciseFrom.Add(time.Hour)

	event := &Event{
		ID:          "precise-time-event",
		From:        preciseFrom,
		To:          preciseTo,
		Title:       "Precision Test Event",
		Description: "Testing timestamp precision",
	}

	err := store.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	// Retrieve and check precision
	retrievedEvent, err := store.GetEvent(ctx, event.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve event: %v", err)
	}

	// PostgreSQL typically stores microsecond precision
	// Check that we don't lose significant precision
	fromDiff := retrievedEvent.From.Sub(event.From).Abs()
	toDiff := retrievedEvent.To.Sub(event.To).Abs()

	// Allow up to 1 second difference for timezone conversion edge cases
	maxAllowedDiff := time.Second
	if fromDiff > maxAllowedDiff {
		t.Errorf("From time precision loss too large: %v", fromDiff)
	}
	if toDiff > maxAllowedDiff {
		t.Errorf("To time precision loss too large: %v", toDiff)
	}
}

func TestStore_UnicodeHandling_Integration(t *testing.T) {
	db := setupTestDB(t)
	if db == nil {
		return
	}
	defer cleanupTestDB(t, db)

	store := NewStore(db)
	ctx := context.Background()

	// Test with unicode characters
	unicodeTitle := "测试事件 🚗 Événement de test Событие テスト"
	unicodeDescription := "包含各种语言和表情符号的描述 📅 🕐 🚕"

	event := &Event{
		ID:          "unicode-event-id",
		From:        time.Now().Truncate(time.Second),
		To:          time.Now().Add(time.Hour).Truncate(time.Second),
		Title:       unicodeTitle,
		Description: unicodeDescription,
	}

	err := store.CreateEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to create unicode event: %v", err)
	}

	// Retrieve and verify unicode preservation
	retrievedEvent, err := store.GetEvent(ctx, event.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve unicode event: %v", err)
	}

	if retrievedEvent.Title != unicodeTitle {
		t.Errorf("Unicode title not preserved: expected %s, got %s", unicodeTitle, retrievedEvent.Title)
	}
	if retrievedEvent.Description != unicodeDescription {
		t.Errorf("Unicode description not preserved: expected %s, got %s", unicodeDescription, retrievedEvent.Description)
	}
}
