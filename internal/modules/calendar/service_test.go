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

// mockStore implements StoreInterface for testing
type mockStore struct {
	events      map[types.ID]*Event
	schedules   map[string]*Schedule // key: "uid:eventID"
	orderEvents map[types.ID]*OrderEvent

	createEventError      error
	getEventError         error
	updateEventError      error
	deleteEventError      error
	listEventsByUserError error
	createScheduleError   error
	getScheduleError      error
	listSchedulesError    error
	createOrderEventError error
	getOrderEventError    error
	deleteOrderEventError error
	listOrderEventsError  error
}

func newMockStore() *mockStore {
	return &mockStore{
		events:      make(map[types.ID]*Event),
		schedules:   make(map[string]*Schedule),
		orderEvents: make(map[types.ID]*OrderEvent),
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

func (m *mockStore) ListEventsByUser(ctx context.Context, uid types.ID) ([]*Event, error) {
	if m.listEventsByUserError != nil {
		return nil, m.listEventsByUserError
	}
	var events []*Event
	for _, sc := range m.schedules {
		if sc.UID == uid {
			if e, ok := m.events[sc.EventID]; ok {
				eventCopy := *e
				events = append(events, &eventCopy)
			}
		}
	}
	return events, nil
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
	return &scheduleCopy, nil
}

func (m *mockStore) ListSchedulesByUser(ctx context.Context, uid types.ID) ([]*Schedule, error) {
	if m.listSchedulesError != nil {
		return nil, m.listSchedulesError
	}
	var schedules []*Schedule
	for _, schedule := range m.schedules {
		if schedule.UID == uid {
			scheduleCopy := *schedule
			schedules = append(schedules, &scheduleCopy)
		}
	}
	return schedules, nil
}

func (m *mockStore) CreateOrderEvent(ctx context.Context, oe *OrderEvent) error {
	if m.createOrderEventError != nil {
		return m.createOrderEventError
	}
	m.orderEvents[oe.ID] = oe
	return nil
}

func (m *mockStore) GetOrderEvent(ctx context.Context, id types.ID) (*OrderEvent, error) {
	if m.getOrderEventError != nil {
		return nil, m.getOrderEventError
	}
	oe, exists := m.orderEvents[id]
	if !exists {
		return nil, ErrNotFound
	}
	oeCopy := *oe
	return &oeCopy, nil
}

func (m *mockStore) DeleteOrderEvent(ctx context.Context, id types.ID) error {
	if m.deleteOrderEventError != nil {
		return m.deleteOrderEventError
	}
	if _, exists := m.orderEvents[id]; !exists {
		return ErrNotFound
	}
	delete(m.orderEvents, id)
	return nil
}

func (m *mockStore) ListOrderEventsByUser(ctx context.Context, uid types.ID) ([]*OrderEvent, error) {
	if m.listOrderEventsError != nil {
		return nil, m.listOrderEventsError
	}
	var orderEvents []*OrderEvent
	for _, oe := range m.orderEvents {
		if oe.UID == uid {
			oeCopy := *oe
			orderEvents = append(orderEvents, &oeCopy)
		}
	}
	return orderEvents, nil
}

func TestCreateEvent_Success(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)

	from := time.Now()
	to := from.Add(time.Hour)
	cmd := CreateEventCommand{
		UID:         "user-123",
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

	// Verify the user was registered as an attendee
	key := store.scheduleKey("user-123", eventID)
	if _, ok := store.schedules[key]; !ok {
		t.Error("Expected a schedule entry for the creator")
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

func TestCreateOrderEvent_Success(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{
		nextOrderID: "test-order-id",
	}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	cmd := CreateOrderEventCommand{
		UID:         "user-123",
		EventID:     "event-456",
		PassengerID: "user-123",
		Pickup:      types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:     types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:    "standard",
	}

	oe, err := svc.CreateOrderEvent(ctx, cmd)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if oe.OrderID != "test-order-id" {
		t.Errorf("Expected OrderID %s, got %s", "test-order-id", oe.OrderID)
	}
	if oe.EventID != cmd.EventID {
		t.Errorf("Expected EventID %s, got %s", cmd.EventID, oe.EventID)
	}
	if oe.UID != cmd.UID {
		t.Errorf("Expected UID %s, got %s", cmd.UID, oe.UID)
	}
	if len(orderSvc.createdOrders) != 1 {
		t.Fatalf("Expected 1 order created, got %d", len(orderSvc.createdOrders))
	}
}

func TestCreateOrderEvent_MultipleOrdersPerEvent(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	uid := types.ID("user-123")
	eventID := types.ID("event-456")

	// Create first order-event (e.g. pickup ride)
	orderSvc.nextOrderID = "order-pickup"
	_, err := svc.CreateOrderEvent(ctx, CreateOrderEventCommand{
		UID: uid, EventID: eventID, PassengerID: uid, RideType: "standard",
	})
	if err != nil {
		t.Fatalf("First CreateOrderEvent failed: %v", err)
	}

	// Create second order-event (e.g. dropoff ride)
	orderSvc.nextOrderID = "order-dropoff"
	_, err = svc.CreateOrderEvent(ctx, CreateOrderEventCommand{
		UID: uid, EventID: eventID, PassengerID: uid, RideType: "standard",
	})
	if err != nil {
		t.Fatalf("Second CreateOrderEvent failed: %v", err)
	}

	// Both order-events should exist
	if len(store.orderEvents) != 2 {
		t.Fatalf("Expected 2 order-events, got %d", len(store.orderEvents))
	}
}

func TestCreateOrderEvent_OrderCreationFails_CleansUp(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{
		createError: errors.New("order service error"),
	}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	_, err := svc.CreateOrderEvent(ctx, CreateOrderEventCommand{
		UID: "user-123", EventID: "event-456", PassengerID: "user-123", RideType: "standard",
	})
	if err == nil {
		t.Fatal("Expected error when order creation fails")
	}
	if len(store.orderEvents) != 0 {
		t.Error("No order-event should be stored when order creation fails")
	}
}

func TestCreateOrderEvent_StoreFails_CancelsOrder(t *testing.T) {
	store := newMockStore()
	store.createOrderEventError = errors.New("store error")
	orderSvc := &mockOrderService{nextOrderID: "test-order-id"}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	_, err := svc.CreateOrderEvent(ctx, CreateOrderEventCommand{
		UID: "user-123", EventID: "event-456", PassengerID: "user-123", RideType: "standard",
	})
	if err == nil {
		t.Fatal("Expected error when store fails")
	}
	// Order should have been created then cancelled
	if len(orderSvc.createdOrders) != 1 {
		t.Errorf("Expected 1 order created, got %d", len(orderSvc.createdOrders))
	}
	if len(orderSvc.cancelledOrders) != 1 {
		t.Errorf("Expected 1 order cancelled (cleanup), got %d", len(orderSvc.cancelledOrders))
	}
}

func TestCancelOrderEvent_Success(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	uid := types.ID("user-123")
	oe := &OrderEvent{
		ID:      "oe-1",
		EventID: "event-456",
		OrderID: "order-789",
		UID:     uid,
	}
	store.orderEvents[oe.ID] = oe

	err := svc.CancelOrderEvent(ctx, CancelOrderEventCommand{
		UID:          uid,
		OrderEventID: "oe-1",
	})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(orderSvc.cancelledOrders) != 1 {
		t.Fatalf("Expected 1 order cancelled, got %d", len(orderSvc.cancelledOrders))
	}
	if _, exists := store.orderEvents["oe-1"]; exists {
		t.Error("Expected order-event to be deleted")
	}
}

func TestCancelOrderEvent_Forbidden(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	oe := &OrderEvent{
		ID:      "oe-1",
		EventID: "event-456",
		OrderID: "order-789",
		UID:     "user-123",
	}
	store.orderEvents[oe.ID] = oe

	err := svc.CancelOrderEvent(ctx, CancelOrderEventCommand{
		UID:          "other-user",
		OrderEventID: "oe-1",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("Expected ErrForbidden, got %v", err)
	}
}

func TestListAllEvents_Success(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	uid := types.ID("user-123")

	// Add events and schedules to mock store
	store.events["event-1"] = &Event{ID: "event-1", Title: "E1", From: time.Now(), To: time.Now().Add(time.Hour)}
	store.events["event-2"] = &Event{ID: "event-2", Title: "E2", From: time.Now(), To: time.Now().Add(time.Hour)}
	store.schedules["user-123:event-1"] = &Schedule{UID: uid, EventID: "event-1"}
	store.schedules["user-123:event-2"] = &Schedule{UID: uid, EventID: "event-2"}

	events, err := svc.ListAllEvents(ctx, uid)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}
}

func TestListAllOrders_Success(t *testing.T) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	uid := types.ID("user-123")
	store.orderEvents["oe-1"] = &OrderEvent{ID: "oe-1", UID: uid, EventID: "event-1", OrderID: "order-1"}
	store.orderEvents["oe-2"] = &OrderEvent{ID: "oe-2", UID: uid, EventID: "event-1", OrderID: "order-2"}
	store.orderEvents["oe-3"] = &OrderEvent{ID: "oe-3", UID: "other-user", EventID: "event-2", OrderID: "order-3"}

	orderEvents, err := svc.ListAllOrders(ctx, uid)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(orderEvents) != 2 {
		t.Fatalf("Expected 2 order-events for user, got %d", len(orderEvents))
	}
	for _, oe := range orderEvents {
		if oe.UID != uid {
			t.Errorf("Expected UID %s, got %s", uid, oe.UID)
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
