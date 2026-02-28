// README: Performance benchmarks for Order module
package order

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"ark/internal/types"
)

// Benchmark service operations
func BenchmarkService_CreateOrder(b *testing.B) {
	store := &Store{}
	pricing := NewMockPricing()
	pricing.SetEstimate("standard", types.Money{Currency: "USD", Amount: 1500})

	service := NewService(store, pricing)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cmd := CreateCommand{
			PassengerID: types.ID(fmt.Sprintf("passenger-%d", i)),
			Pickup:      types.Point{Lat: 37.7749, Lng: -122.4194},
			Dropoff:     types.Point{Lat: 37.7849, Lng: -122.4094},
			RideType:    "standard",
		}

		_, err := service.Create(ctx, cmd)
		if err != nil {
			b.Fatalf("Create failed: %v", err)
		}
	}
}

func BenchmarkService_AcceptOrder(b *testing.B) {
	store := &Store{}
	pricing := NewMockPricing()
	service := NewService(store, pricing)
	ctx := context.Background()

	// Pre-create orders to accept
	orders := make([]types.ID, b.N)
	for i := 0; i < b.N; i++ {
		order := &Order{
			ID:            types.ID(fmt.Sprintf("order-%d", i)),
			PassengerID:   types.ID(fmt.Sprintf("passenger-%d", i)),
			Status:        StatusWaiting,
			StatusVersion: 1,
			Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
			Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
			RideType:      "standard",
			EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
			CreatedAt:     time.Now(),
		}
		store.Create(ctx, order)
		orders[i] = order.ID
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cmd := AcceptCommand{
			OrderID:  orders[i],
			DriverID: types.ID(fmt.Sprintf("driver-%d", i)),
		}

		err := service.Accept(ctx, cmd)
		if err != nil {
			b.Fatalf("Accept failed: %v", err)
		}
	}
}

func BenchmarkService_CancelOrder(b *testing.B) {
	store := &Store{}
	pricing := NewMockPricing()
	service := NewService(store, pricing)
	ctx := context.Background()

	// Pre-create orders to cancel
	orders := make([]types.ID, b.N)
	for i := 0; i < b.N; i++ {
		order := &Order{
			ID:            types.ID(fmt.Sprintf("order-%d", i)),
			PassengerID:   types.ID(fmt.Sprintf("passenger-%d", i)),
			Status:        StatusWaiting,
			StatusVersion: 1,
			Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
			Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
			RideType:      "standard",
			EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
			CreatedAt:     time.Now(),
		}
		store.Create(ctx, order)
		orders[i] = order.ID
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cmd := CancelCommand{
			OrderID:   orders[i],
			ActorType: "passenger",
			Reason:    "benchmark test",
		}

		err := service.Cancel(ctx, cmd)
		if err != nil {
			b.Fatalf("Cancel failed: %v", err)
		}
	}
}

func BenchmarkNewID(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		id := newID()
		if id == "" {
			b.Fatal("Generated empty ID")
		}
	}
}

func BenchmarkNewID_Parallel(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := newID()
			if id == "" {
				b.Fatal("Generated empty ID")
			}
		}
	})
}

func BenchmarkService_ConcurrentCreate(b *testing.B) {
	store := &Store{}
	pricing := NewMockPricing()
	pricing.SetEstimate("standard", types.Money{Currency: "USD", Amount: 1500})

	service := NewService(store, pricing)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cmd := CreateCommand{
				PassengerID: types.ID(fmt.Sprintf("passenger-%d-%d", b.N, i)),
				Pickup:      types.Point{Lat: 37.7749, Lng: -122.4194},
				Dropoff:     types.Point{Lat: 37.7849, Lng: -122.4094},
				RideType:    "standard",
			}

			_, err := service.Create(ctx, cmd)
			if err != nil {
				b.Fatalf("Create failed: %v", err)
			}
			i++
		}
	})
}

func BenchmarkCanTransition(b *testing.B) {
	transitions := []struct {
		from, to Status
	}{
		{StatusWaiting, StatusApproaching},
		{StatusApproaching, StatusArrived},
		{StatusArrived, StatusDriving},
		{StatusDriving, StatusComplete},
		{StatusWaiting, StatusCancelled},
		{StatusComplete, StatusWaiting}, // Invalid transition
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		transition := transitions[i%len(transitions)]
		_ = CanTransition(transition.from, transition.to)
	}
}

func BenchmarkMockStore_Operations(b *testing.B) {
	store := &Store{}
	ctx := context.Background()

	// Pre-populate with some orders
	baseOrders := make([]*Order, 100)
	for i := 0; i < 100; i++ {
		order := &Order{
			ID:            types.ID(fmt.Sprintf("base-order-%d", i)),
			PassengerID:   types.ID(fmt.Sprintf("passenger-%d", i%10)), // 10 passengers
			Status:        StatusWaiting,
			StatusVersion: 1,
			Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
			Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
			RideType:      "standard",
			EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
			CreatedAt:     time.Now(),
		}
		store.Create(ctx, order)
		baseOrders[i] = order
	}

	b.Run("CreateOrder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			order := &Order{
				ID:            types.ID(fmt.Sprintf("bench-create-%d", i)),
				PassengerID:   types.ID(fmt.Sprintf("passenger-%d", i%1000)),
				Status:        StatusWaiting,
				StatusVersion: 1,
				Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
				Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
				RideType:      "standard",
				EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
				CreatedAt:     time.Now(),
			}

			err := store.Create(ctx, order)
			if err != nil {
				b.Fatalf("Create failed: %v", err)
			}
		}
	})

	b.Run("GetOrder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			orderID := baseOrders[i%len(baseOrders)].ID
			_, err := store.Get(ctx, orderID)
			if err != nil {
				b.Fatalf("Get failed: %v", err)
			}
		}
	})

	b.Run("UpdateOrder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			order := baseOrders[i%len(baseOrders)]
			updatedOrder := *order
			updatedOrder.Status = StatusApproaching
			updatedOrder.StatusVersion = 2
			driverID := types.ID(fmt.Sprintf("driver-%d", i))
			updatedOrder.DriverID = &driverID

			// Use UpdateStatus instead of Update
			success, err := store.UpdateStatus(ctx, updatedOrder.ID, StatusWaiting, StatusApproaching, updatedOrder.StatusVersion, &driverID)
			if err != nil {
				b.Fatalf("UpdateStatus failed: %v", err)
			}
			if !success {
				b.Fatalf("UpdateStatus returned false - optimistic lock failed")
			}

			// Reset for next iteration
			order.Status = StatusWaiting
			order.StatusVersion = 1
			order.DriverID = nil
		}
	})

	b.Run("HasActiveByPassenger", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			passengerID := types.ID(fmt.Sprintf("passenger-%d", i%10))
			_, err := store.HasActiveByPassenger(ctx, passengerID)
			if err != nil {
				b.Fatalf("HasActiveByPassenger failed: %v", err)
			}
		}
	})
}

func BenchmarkOrder_Creation(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		order := Order{
			ID:            types.ID(fmt.Sprintf("order-%d", i)),
			PassengerID:   types.ID(fmt.Sprintf("passenger-%d", i)),
			Status:        StatusWaiting,
			StatusVersion: 1,
			Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
			Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
			RideType:      "standard",
			EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
			CreatedAt:     time.Now(),
		}

		// Use the order to prevent optimization
		if order.ID == "" {
			b.Fatal("Empty order ID")
		}
	}
}

func BenchmarkOrder_StatusCheck(b *testing.B) {
	order := Order{Status: StatusWaiting}
	statuses := []Status{
		StatusWaiting, StatusApproaching, StatusArrived,
		StatusDriving, StatusComplete, StatusCancelled,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		status := statuses[i%len(statuses)]
		_ = (order.Status == status)
	}
}

func BenchmarkMockPricing_Estimate(b *testing.B) {
	pricing := NewMockPricing()
	pricing.SetEstimate("standard", types.Money{Currency: "USD", Amount: 1500})
	pricing.SetEstimate("premium", types.Money{Currency: "USD", Amount: 2500})

	ctx := context.Background()
	distances := []float64{1.0, 2.5, 5.0, 10.0, 25.0}
	rideTypes := []string{"standard", "premium"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		distance := distances[i%len(distances)]
		rideType := rideTypes[i%len(rideTypes)]

		_, err := pricing.Estimate(ctx, distance, rideType)
		if err != nil {
			b.Fatalf("Estimate failed: %v", err)
		}
	}
}

// Memory usage benchmark
func BenchmarkService_CreateOrder_Memory(b *testing.B) {
	store := &Store{}
	pricing := NewMockPricing()
	pricing.SetEstimate("standard", types.Money{Currency: "USD", Amount: 1500})

	service := NewService(store, pricing)
	ctx := context.Background()

	// Measure memory usage
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cmd := CreateCommand{
			PassengerID: types.ID(fmt.Sprintf("passenger-%d", i)),
			Pickup:      types.Point{Lat: 37.7749, Lng: -122.4194},
			Dropoff:     types.Point{Lat: 37.7849, Lng: -122.4094},
			RideType:    "standard",
		}

		_, err := service.Create(ctx, cmd)
		if err != nil {
			b.Fatalf("Create failed: %v", err)
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
}

// Scalability benchmark with varying data sizes
func BenchmarkService_LargeScale(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			store := &Store{}
			pricing := NewMockPricing()
			pricing.SetEstimate("standard", types.Money{Currency: "USD", Amount: 1500})

			service := NewService(store, pricing)
			ctx := context.Background()

			// Pre-populate with orders
			for i := 0; i < size; i++ {
				order := &Order{
					ID:            types.ID(fmt.Sprintf("existing-%d", i)),
					PassengerID:   types.ID(fmt.Sprintf("passenger-%d", i%100)), // 100 passengers
					Status:        StatusWaiting,
					StatusVersion: 1,
					Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
					Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
					RideType:      "standard",
					EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
					CreatedAt:     time.Now(),
				}
				store.Create(ctx, order)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				cmd := CreateCommand{
					PassengerID: types.ID(fmt.Sprintf("new-passenger-%d", i)),
					Pickup:      types.Point{Lat: 37.7749, Lng: -122.4194},
					Dropoff:     types.Point{Lat: 37.7849, Lng: -122.4094},
					RideType:    "standard",
				}

				_, err := service.Create(ctx, cmd)
				if err != nil {
					b.Fatalf("Create failed: %v", err)
				}
			}
		})
	}
}
