// README: Benchmark tests for the calendar module to measure performance characteristics.
package calendar

import (
	"context"
	"fmt"
	"testing"
	"time"

	"ark/internal/types"
)

func BenchmarkService_CreateEvent(b *testing.B) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	cmd := CreateEventCommand{
		From:        time.Now(),
		To:          time.Now().Add(time.Hour),
		Title:       "Benchmark Event",
		Description: "Benchmark Description",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.CreateEvent(ctx, cmd)
		if err != nil {
			b.Fatalf("CreateEvent failed: %v", err)
		}
	}
}

func BenchmarkService_CreateOrderEvent(b *testing.B) {
	store := newMockStore()
	orderSvc := &mockOrderService{
		nextOrderID: "bench-order-id",
	}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	cmd := CreateOrderEventCommand{
		UID:         "bench-user",
		EventID:     "bench-event",
		PassengerID: "bench-passenger",
		Pickup:      types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:     types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:    "standard",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset state for each iteration
		store.orderEvents = make(map[types.ID]*OrderEvent)
		orderSvc.createdOrders = nil

		_, err := svc.CreateOrderEvent(ctx, cmd)
		if err != nil {
			b.Fatalf("CreateOrderEvent failed: %v", err)
		}
	}
}

func BenchmarkService_ListAllOrders(b *testing.B) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	uid := types.ID("bench-user")

	// Pre-populate with many order-events
	for i := 0; i < 1000; i++ {
		id := types.ID(fmt.Sprintf("oe-%d", i))
		store.orderEvents[id] = &OrderEvent{
			ID:      id,
			UID:     uid,
			EventID: types.ID(fmt.Sprintf("event-%d", i%100)),
			OrderID: types.ID(fmt.Sprintf("order-%d", i)),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orderEvents, err := svc.ListAllOrders(ctx, uid)
		if err != nil {
			b.Fatalf("ListAllOrders failed: %v", err)
		}
		if len(orderEvents) != 1000 {
			b.Fatalf("Expected 1000 order-events, got %d", len(orderEvents))
		}
	}
}

func BenchmarkNewID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = newID()
	}
}

func BenchmarkNewID_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = newID()
		}
	})
}

// Benchmark memory allocations
func BenchmarkService_CreateEvent_Memory(b *testing.B) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	cmd := CreateEventCommand{
		From:        time.Now(),
		To:          time.Now().Add(time.Hour),
		Title:       "Memory Test Event",
		Description: "Memory Test Description",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := svc.CreateEvent(ctx, cmd)
		if err != nil {
			b.Fatalf("CreateEvent failed: %v", err)
		}
	}
}

// Benchmark concurrent access
func BenchmarkService_ConcurrentCreateEvent(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine gets its own store to avoid race conditions
		store := newMockStore()
		orderSvc := &mockOrderService{}
		svc := NewService(store, orderSvc)
		ctx := context.Background()

		for pb.Next() {
			cmd := CreateEventCommand{
				From:        time.Now(),
				To:          time.Now().Add(time.Hour),
				Title:       "Concurrent Event",
				Description: "Concurrent Description",
			}

			_, err := svc.CreateEvent(ctx, cmd)
			if err != nil {
				b.Fatalf("CreateEvent failed: %v", err)
			}
		}
	})
}

func BenchmarkService_LargeOrderEventList(b *testing.B) {
	store := newMockStore()
	orderSvc := &mockOrderService{}
	svc := NewService(store, orderSvc)
	ctx := context.Background()

	// Test with different sizes
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			// Reset and populate
			store.orderEvents = make(map[types.ID]*OrderEvent)
			uid := types.ID("bench-user")

			for i := 0; i < size; i++ {
				id := types.ID(fmt.Sprintf("oe-%d", i))
				store.orderEvents[id] = &OrderEvent{
					ID:      id,
					UID:     uid,
					EventID: types.ID(fmt.Sprintf("event-%d", i%100)),
					OrderID: types.ID(fmt.Sprintf("order-%d", i)),
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				orderEvents, err := svc.ListAllOrders(ctx, uid)
				if err != nil {
					b.Fatalf("ListAllOrders failed: %v", err)
				}
				if len(orderEvents) != size {
					b.Fatalf("Expected %d order-events, got %d", size, len(orderEvents))
				}
			}
		})
	}
}

func BenchmarkMockStore_Operations(b *testing.B) {
	store := newMockStore()
	ctx := context.Background()

	b.Run("CreateEvent", func(b *testing.B) {
		event := &Event{
			ID:          "bench-event",
			From:        time.Now(),
			To:          time.Now().Add(time.Hour),
			Title:       "Bench Event",
			Description: "Bench Description",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := store.CreateEvent(ctx, event)
			if err != nil {
				b.Fatalf("CreateEvent failed: %v", err)
			}
		}
	})

	b.Run("GetEvent", func(b *testing.B) {
		// Pre-populate
		event := &Event{
			ID:          "bench-event",
			From:        time.Now(),
			To:          time.Now().Add(time.Hour),
			Title:       "Bench Event",
			Description: "Bench Description",
		}
		store.CreateEvent(ctx, event)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := store.GetEvent(ctx, event.ID)
			if err != nil {
				b.Fatalf("GetEvent failed: %v", err)
			}
		}
	})

	b.Run("CreateSchedule", func(b *testing.B) {
		schedule := &Schedule{
			UID:     "bench-user",
			EventID: "bench-event",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Reset state
			store.schedules = make(map[string]*Schedule)

			err := store.CreateSchedule(ctx, schedule)
			if err != nil {
				b.Fatalf("CreateSchedule failed: %v", err)
			}
		}
	})
}
