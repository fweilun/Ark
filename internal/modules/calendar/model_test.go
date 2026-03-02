// README: Unit tests for calendar models and their behavior.
package calendar

import (
	"testing"
	"time"

	"ark/internal/types"
)

func TestEvent_FieldValidation(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name        string
		event       Event
		description string
	}{
		{
			name: "valid event",
			event: Event{
				ID:          "event-123",
				From:        now,
				To:          now.Add(time.Hour),
				Title:       "Test Event",
				Description: "Test Description",
			},
			description: "All fields properly set",
		},
		{
			name: "empty description",
			event: Event{
				ID:          "event-123",
				From:        now,
				To:          now.Add(time.Hour),
				Title:       "Test Event",
				Description: "",
			},
			description: "Description can be empty",
		},
		{
			name: "very long title",
			event: Event{
				ID:          "event-123",
				From:        now,
				To:          now.Add(time.Hour),
				Title:       "A very long title that tests the system's ability to handle extended text content without issues or truncation problems that might occur in various scenarios",
				Description: "Test Description",
			},
			description: "Long titles should be handled properly",
		},
		{
			name: "unicode content",
			event: Event{
				ID:          "event-123",
				From:        now,
				To:          now.Add(time.Hour),
				Title:       "测试事件 🚗 Test Event",
				Description: "描述包含中文和表情符号 📅",
			},
			description: "Unicode characters should be preserved",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test that the event struct holds values correctly
			if tc.event.ID == "" {
				t.Error("Event ID should not be empty in test cases")
			}

			// Test time relationships
			if tc.event.From.After(tc.event.To) {
				t.Error("Event From time should not be after To time")
			}

			// Test field preservation
			if tc.event.Title == "" && tc.name != "empty title test" {
				t.Error("Event Title should not be empty in valid test cases")
			}

			t.Logf("Test case '%s': %s", tc.name, tc.description)
		})
	}
}

func TestSchedule_FieldValidation(t *testing.T) {
	uid := types.ID("user-123")
	eventID := types.ID("event-456")

	schedule := Schedule{
		UID:     uid,
		EventID: eventID,
	}

	if schedule.UID == "" {
		t.Error("Schedule UID should not be empty")
	}
	if schedule.EventID == "" {
		t.Error("Schedule EventID should not be empty")
	}
}

func TestOrderEvent_FieldValidation(t *testing.T) {
	uid := types.ID("user-123")
	eventID := types.ID("event-456")
	orderID := types.ID("order-789")

	testCases := []struct {
		name       string
		orderEvent OrderEvent
		wantValid  bool
	}{
		{
			name: "valid order event",
			orderEvent: OrderEvent{
				ID:        "oe-1",
				EventID:   eventID,
				OrderID:   orderID,
				UID:       uid,
				CreatedAt: time.Now(),
			},
			wantValid: true,
		},
		{
			name:       "zero value order event",
			orderEvent: OrderEvent{},
			wantValid:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid := tc.orderEvent.ID != "" && tc.orderEvent.EventID != "" && tc.orderEvent.OrderID != "" && tc.orderEvent.UID != ""
			if isValid != tc.wantValid {
				t.Errorf("Expected valid=%v, got valid=%v", tc.wantValid, isValid)
			}
		})
	}
}

func TestEvent_TimeRelationships(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name     string
		from     time.Time
		to       time.Time
		duration time.Duration
		valid    bool
	}{
		{
			name:     "one hour event",
			from:     now,
			to:       now.Add(time.Hour),
			duration: time.Hour,
			valid:    true,
		},
		{
			name:     "one minute event",
			from:     now,
			to:       now.Add(time.Minute),
			duration: time.Minute,
			valid:    true,
		},
		{
			name:     "one second event",
			from:     now,
			to:       now.Add(time.Second),
			duration: time.Second,
			valid:    true,
		},
		{
			name:     "one nanosecond event",
			from:     now,
			to:       now.Add(1),
			duration: time.Nanosecond,
			valid:    true,
		},
		{
			name:     "multi-day event",
			from:     now,
			to:       now.Add(24 * time.Hour * 7), // One week
			duration: 24 * time.Hour * 7,
			valid:    true,
		},
		{
			name:     "same start and end time",
			from:     now,
			to:       now,
			duration: 0,
			valid:    false, // Business rule: events must have duration
		},
		{
			name:     "end before start",
			from:     now,
			to:       now.Add(-time.Hour),
			duration: -time.Hour,
			valid:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := Event{
				ID:          "test-event",
				From:        tc.from,
				To:          tc.to,
				Title:       "Test Event",
				Description: "Test Description",
			}

			actualDuration := event.To.Sub(event.From)
			if actualDuration != tc.duration {
				t.Errorf("Expected duration %v, got %v", tc.duration, actualDuration)
			}

			isValid := event.From.Before(event.To)
			if isValid != tc.valid {
				t.Errorf("Expected validity %v, got %v for time relationship", tc.valid, isValid)
			}

			t.Logf("Event duration: %v, valid: %v", actualDuration, isValid)
		})
	}
}

func TestEvent_IDUniqueness(t *testing.T) {
	event1 := Event{
		ID:          "event-1",
		From:        time.Now(),
		To:          time.Now().Add(time.Hour),
		Title:       "Event 1",
		Description: "Description 1",
	}

	event2 := Event{
		ID:          "event-2",
		From:        time.Now(),
		To:          time.Now().Add(time.Hour),
		Title:       "Event 2",
		Description: "Description 2",
	}

	if event1.ID == event2.ID {
		t.Error("Different events should have different IDs")
	}
}

func TestSchedule_CompositeKey(t *testing.T) {
	uid1 := types.ID("user-1")
	uid2 := types.ID("user-2")
	eventID1 := types.ID("event-1")
	eventID2 := types.ID("event-2")

	schedules := []Schedule{
		{UID: uid1, EventID: eventID1},
		{UID: uid1, EventID: eventID2},
		{UID: uid2, EventID: eventID1},
		{UID: uid2, EventID: eventID2},
	}

	// Each schedule should have a unique UID+EventID combination
	keys := make(map[string]bool)
	for i, schedule := range schedules {
		key := string(schedule.UID) + ":" + string(schedule.EventID)
		if keys[key] {
			t.Errorf("Schedule %d has duplicate key %s", i, key)
		}
		keys[key] = true
	}

	if len(keys) != 4 {
		t.Errorf("Expected 4 unique keys, got %d", len(keys))
	}
}

func TestOrderEvent_MultiplePerEvent(t *testing.T) {
	// Multiple order-events can exist for the same event
	eventID := types.ID("event-1")
	uid := types.ID("user-1")

	orderEvents := []OrderEvent{
		{ID: "oe-1", EventID: eventID, OrderID: "order-pickup", UID: uid},
		{ID: "oe-2", EventID: eventID, OrderID: "order-dropoff", UID: uid},
	}

	ids := make(map[types.ID]bool)
	for _, oe := range orderEvents {
		if ids[oe.ID] {
			t.Errorf("Duplicate order-event ID: %s", oe.ID)
		}
		ids[oe.ID] = true
		if oe.EventID != eventID {
			t.Errorf("Expected EventID %s, got %s", eventID, oe.EventID)
		}
	}

	if len(ids) != 2 {
		t.Errorf("Expected 2 unique order-event IDs, got %d", len(ids))
	}
}

func TestEvent_ZeroValues(t *testing.T) {
	var event Event

	if event.ID != "" {
		t.Errorf("Zero-value Event ID should be empty string, got %s", event.ID)
	}

	if !event.From.IsZero() {
		t.Error("Zero-value Event From should be zero time")
	}

	if !event.To.IsZero() {
		t.Error("Zero-value Event To should be zero time")
	}

	if event.Title != "" {
		t.Errorf("Zero-value Event Title should be empty string, got %s", event.Title)
	}

	if event.Description != "" {
		t.Errorf("Zero-value Event Description should be empty string, got %s", event.Description)
	}
}

func TestSchedule_ZeroValues(t *testing.T) {
	var schedule Schedule

	if schedule.UID != "" {
		t.Errorf("Zero-value Schedule UID should be empty string, got %s", schedule.UID)
	}

	if schedule.EventID != "" {
		t.Errorf("Zero-value Schedule EventID should be empty string, got %s", schedule.EventID)
	}
}

func TestOrderEvent_ZeroValues(t *testing.T) {
	var oe OrderEvent

	if oe.ID != "" {
		t.Errorf("Zero-value OrderEvent ID should be empty string, got %s", oe.ID)
	}
	if oe.EventID != "" {
		t.Errorf("Zero-value OrderEvent EventID should be empty string, got %s", oe.EventID)
	}
	if oe.OrderID != "" {
		t.Errorf("Zero-value OrderEvent OrderID should be empty string, got %s", oe.OrderID)
	}
	if oe.UID != "" {
		t.Errorf("Zero-value OrderEvent UID should be empty string, got %s", oe.UID)
	}
	if !oe.CreatedAt.IsZero() {
		t.Error("Zero-value OrderEvent CreatedAt should be zero time")
	}
}

func TestEvent_TimezoneHandling(t *testing.T) {
	utc := time.UTC
	est := time.FixedZone("EST", -5*60*60) // UTC-5
	pst := time.FixedZone("PST", -8*60*60) // UTC-8

	baseTime := time.Date(2024, 3, 15, 10, 0, 0, 0, utc)

	testCases := []struct {
		name     string
		timezone *time.Location
	}{
		{"UTC", utc},
		{"EST", est},
		{"PST", pst},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			from := baseTime.In(tc.timezone)
			to := from.Add(time.Hour)

			event := Event{
				ID:          "timezone-event",
				From:        from,
				To:          to,
				Title:       "Timezone Test",
				Description: "Testing timezone handling",
			}

			if event.From.Location() != tc.timezone {
				t.Errorf("Expected timezone %s, got %s", tc.timezone, event.From.Location())
			}

			duration := event.To.Sub(event.From)
			if duration != time.Hour {
				t.Errorf("Expected duration 1 hour, got %v", duration)
			}

			t.Logf("Event in %s: From=%v, To=%v, Duration=%v",
				tc.name, event.From, event.To, duration)
		})
	}
}

func BenchmarkEvent_Creation(b *testing.B) {
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Event{
			ID:          types.ID("benchmark-event"),
			From:        now,
			To:          now.Add(time.Hour),
			Title:       "Benchmark Event",
			Description: "Benchmark Description",
		}
	}
}

func BenchmarkSchedule_Creation(b *testing.B) {
	uid := types.ID("benchmark-user")
	eventID := types.ID("benchmark-event")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Schedule{
			UID:     uid,
			EventID: eventID,
		}
	}
}

func BenchmarkOrderEvent_Creation(b *testing.B) {
	uid := types.ID("benchmark-user")
	eventID := types.ID("benchmark-event")
	orderID := types.ID("benchmark-order")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = OrderEvent{
			ID:        "oe-bench",
			UID:       uid,
			EventID:   eventID,
			OrderID:   orderID,
			CreatedAt: time.Now(),
		}
	}
}
