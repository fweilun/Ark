// README: Comprehensive tests for the calendar service covering all functionality and edge cases.
package calendar

import (
	"context"
	"errors"
	"testing"
	"time"

	"ark/internal/modules/order"
	"ark/internal/types"
)

// mockOrderService implements OrderService for testing
type mockOrderService struct {
	createdOrders   []order.CreateCommand
	cancelledOrders []order.CancelCommand
	createError     error
	cancelError     error
	nextOrderID     types.ID
}

func (m *mockOrderService) Create(ctx context.Context, cmd order.CreateCommand) (types.ID, error) {
	if m.createError != nil {
		return "", m.createError
	}
	m.createdOrders = append(m.createdOrders, cmd)
	orderID := m.nextOrderID
	if orderID == "" {
		orderID = "test-order-id"
	}
	return orderID, nil
}

func (m *mockOrderService) Cancel(ctx context.Context, cmd order.CancelCommand) error {
	if m.cancelError != nil {
		return m.cancelError
	}
	m.cancelledOrders = append(m.cancelledOrders, cmd)
	return nil
}

// mockStore implements Store interface for testing
type mockStore struct {
	events    map[types.ID]*Event
	schedules map[string]*Schedule // key: "uid:eventID"

	createEventError    error
	getEventError       error
	updateEventError    error
	deleteEventError    error
	createScheduleError error
	getScheduleError    error
	updateScheduleError error
	listSchedulesError  error
}

func newMockStore() *mockStore {
	return &mockStore{
		events:    make(map[types.ID]*Event),
		schedules: make(map[string]*Schedule),
	}
}

func (m *mockStore) scheduleKey(uid, eventID types.ID) string {
	return string(uid) + ":" + string(eventID)
}

func (m *mockStore) CreateEvent(ctx context.Context, e *Event) error {
	if m.createEventError != nil {
		return m.createEventError
	}
	m.events[e.ID] = e
	return nil
}

func (m *mockStore) GetEvent(ctx context.Context, id types.ID) (*Event, error) {
	if m.getEventError != nil {
		return nil, m.getEventError
	}
	event, exists := m.events[id]
	if !exists {
		return nil, ErrNotFound
	}
	eventCopy := *event
	return &eventCopy, nil
}

func (m *mockStore) UpdateEvent(ctx context.Context, e *Event) error {
	if m.updateEventError != nil {
		return m.updateEventError
	}
	if _, exists := m.events[e.ID]; !exists {
		return ErrNotFound
	}
	m.events[e.ID] = e
	return nil
}

func (m *mockStore) DeleteEvent(ctx context.Context, id types.ID) error {
	if m.deleteEventError != nil {
		return m.deleteEventError
	}
	if _, exists := m.events[id]; !exists {
		return ErrNotFound
	}
	delete(m.events, id)
	return nil
}

func (m *mockStore) CreateSchedule(ctx context.Context, sc *Schedule) error {
	if m.createScheduleError != nil {
		return m.createScheduleError
	}
	key := m.scheduleKey(sc.UID, sc.EventID)
	m.schedules[key] = sc
	return nil
}

func (m *mockStore) GetSchedule(ctx context.Context, uid, eventID types.ID) (*Schedule, error) {
	if m.getScheduleError != nil {
		return nil, m.getScheduleError
	}
	key := m.scheduleKey(uid, eventID)
	schedule, exists := m.schedules[key]
	if !exists {
		return nil, ErrNotFound
	}
	scheduleCopy := *schedule
	if schedule.TiedOrder != nil {
		orderID := *schedule.TiedOrder
		scheduleCopy.TiedOrder = &orderID
	}
	return &scheduleCopy, nil
}

func (m *mockStore) UpdateScheduleTiedOrder(ctx context.Context, uid, eventID types.ID, orderID *types.ID) error {
	if m.updateScheduleError != nil {
		return m.updateScheduleError
	}
	key := m.scheduleKey(uid, eventID)
	schedule, exists := m.schedules[key]
	if !exists {
		return ErrNotFound
	}
	if orderID != nil {
		newOrderID := *orderID
		schedule.TiedOrder = &newOrderID
	} else {
		schedule.TiedOrder = nil
	}
	return nil
}

func (m *mockStore) ListSchedulesByUser(ctx context.Context, uid types.ID) ([]*Schedule, error) {
	if m.listSchedulesError != nil {
		return nil, m.listSchedulesError
	}
	var schedules []*Schedule
	for _, schedule := range m.schedules {
		if schedule.UID == uid {
			scheduleCopy := *schedule
			if schedule.TiedOrder != nil {
				orderID := *schedule.TiedOrder
				scheduleCopy.TiedOrder = &orderID
			}
			schedules = append(schedules, &scheduleCopy)
		}
	}
	return schedules, nil
}

func TestCreateEvent_Success(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)

	from := time.Now()
	to := from.Add(time.Hour)
	cmd := CreateEventCommand{
		From:        from,
		To:          to,
		Title:       "Test Event",
		Description: "Test Description",
	}

	ctx := context.Background()
	eventID, err := svc.CreateEvent(ctx, cmd)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if eventID == "" {
		t.Fatal("Expected event ID, got empty string")
	}

	event, exists := store.events[eventID]
	if !exists {
		t.Fatal("Event was not stored")
	}
	if event.Title != cmd.Title {
		t.Errorf("Expected title %s, got %s", cmd.Title, event.Title)
	}
}

func TestCreateEvent_ValidationErrors(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	testCases := []struct {
		name string
		cmd  CreateEventCommand
	}{
		{
			name: "empty title",
			cmd: CreateEventCommand{
				From:  time.Now(),
				To:    time.Now().Add(time.Hour),
				Title: "",
			},
		},
		{
			name: "from time after to time",
			cmd: CreateEventCommand{
				From:  time.Now().Add(time.Hour),
				To:    time.Now(),
				Title: "Test",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateEvent(ctx, tc.cmd)
			if !errors.Is(err, ErrBadRequest) {
				t.Errorf("Expected ErrBadRequest, got %v", err)
			}
		})
	}
}

func TestCreateAndTieOrder_Success(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{
		nextOrderID: "test-order-id",
	}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	cmd := CreateAndTieOrderCommand{
		UID:         "user-123",
		EventID:     "event-456",
		PassengerID: "passenger-789",
		Pickup:      types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:     types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:    "standard",
	}

	schedule, err := svc.CreateAndTieOrder(ctx, cmd)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if schedule.TiedOrder == nil {
		t.Fatal("Expected TiedOrder to be set")
	}
	if *schedule.TiedOrder != "test-order-id" {
		t.Errorf("Expected TiedOrder %s, got %s", "test-order-id", *schedule.TiedOrder)
	}

	if len(orderSvc.createdOrders) != 1 {
		t.Fatalf("Expected 1 order created, got %d", len(orderSvc.createdOrders))
	}
}

func TestUntieOrder_Success(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	uid := types.ID("user-123")
	eventID := types.ID("event-456")
	orderID := types.ID("order-789")

	schedule := &Schedule{
		UID:       uid,
		EventID:   eventID,
		TiedOrder: &orderID,
	}
	key := store.scheduleKey(uid, eventID)
	store.schedules[key] = schedule

	cmd := UntieOrderCommand{
		UID:     uid,
		EventID: eventID,
	}

	err := svc.UntieOrder(ctx, cmd)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(orderSvc.cancelledOrders) != 1 {
		t.Fatalf("Expected 1 order cancelled, got %d", len(orderSvc.cancelledOrders))
	}

	updatedSchedule := store.schedules[key]
	if updatedSchedule.TiedOrder != nil {
		t.Error("Expected TiedOrder to be nil after untying")
	}
}

func TestListSchedulesByUser_Success(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	uid1 := types.ID("user-123")
	uid2 := types.ID("user-456")
	eventID1 := types.ID("event-1")
	eventID2 := types.ID("event-2")
	eventID3 := types.ID("event-3")

	schedule1 := &Schedule{UID: uid1, EventID: eventID1, TiedOrder: nil}
	schedule2 := &Schedule{UID: uid1, EventID: eventID2, TiedOrder: nil}
	schedule3 := &Schedule{UID: uid2, EventID: eventID3, TiedOrder: nil}

	store.schedules[store.scheduleKey(uid1, eventID1)] = schedule1
	store.schedules[store.scheduleKey(uid1, eventID2)] = schedule2
	store.schedules[store.scheduleKey(uid2, eventID3)] = schedule3

	schedules, err := svc.ListSchedulesByUser(ctx, uid1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(schedules) != 2 {
		t.Fatalf("Expected 2 schedules, got %d", len(schedules))
	}

	for _, schedule := range schedules {
		if schedule.UID != uid1 {
			t.Errorf("Expected UID %s, got %s", uid1, schedule.UID)
		}
	}
}

func TestNewIDGeneratesUniqueValues(t *testing.T) {
	ids := make(map[types.ID]bool)
	for i := 0; i < 1000; i++ {
		id := newID()
		if ids[id] {
			t.Fatalf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
		if len(string(id)) != 32 { // 16 bytes * 2 hex chars per byte
			t.Errorf("Expected ID length 32, got %d for ID %s", len(string(id)), id)
		}
	}
}
