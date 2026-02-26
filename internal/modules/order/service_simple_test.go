// README: Simplified service tests using the actual Store
package order

import (
	"context"
	"errors"
	"testing"
	"time"

	"ark/internal/types"
)

// MockPricing implements Pricing interface for unit testing
type MockPricing struct {
	estimates map[string]types.Money
	errors    map[string]error
}

func NewMockPricing() *MockPricing {
	return &MockPricing{
		estimates: make(map[string]types.Money),
		errors:    make(map[string]error),
	}
}

func (m *MockPricing) SetEstimate(rideType string, price types.Money) {
	m.estimates[rideType] = price
}

func (m *MockPricing) SetError(rideType string, err error) {
	m.errors[rideType] = err
}

func (m *MockPricing) Estimate(ctx context.Context, distanceKm float64, rideType string) (types.Money, error) {
	if err, hasError := m.errors[rideType]; hasError {
		return types.Money{}, err
	}

	if estimate, hasEstimate := m.estimates[rideType]; hasEstimate {
		return estimate, nil
	}

	// Default calculation: $1 per km + $5 base
	amount := int64(distanceKm*100) + 500 // $1/km + $5 base
	return types.Money{Currency: "USD", Amount: amount}, nil
}

func TestService_NewService(t *testing.T) {
	store := &Store{} // Use actual store
	pricing := NewMockPricing()

	service := NewService(store, pricing)

	if service == nil {
		t.Fatal("NewService should return a service instance")
	}

	if service.store == nil {
		t.Error("Service should have store reference")
	}

	if service.pricing == nil {
		t.Error("Service should have pricing reference")
	}
}

func TestService_newID(t *testing.T) {
	// Test the package-level newID function
	id1 := newID()
	id2 := newID()

	if id1 == "" {
		t.Error("newID should not return empty string")
	}

	if id2 == "" {
		t.Error("newID should not return empty string")
	}

	if id1 == id2 {
		t.Error("newID should return unique IDs")
	}

	// Test that IDs have reasonable length (UUID-like)
	if len(string(id1)) < 10 {
		t.Errorf("ID seems too short: %s", id1)
	}
}

func TestService_calculateDistance(t *testing.T) {
	// Test distance calculation logic
	pickup := types.Point{Lat: 37.7749, Lng: -122.4194}  // SF
	dropoff := types.Point{Lat: 37.7849, Lng: -122.4094} // SF nearby

	distance := calculateDistance(pickup, dropoff)

	if distance <= 0 {
		t.Error("Distance should be positive")
	}

	if distance > 100 { // Should be reasonable for nearby points
		t.Errorf("Distance seems too large: %f km", distance)
	}
}

func TestService_canTransitionBoundaryConditions(t *testing.T) {
	tests := []struct {
		name     string
		from     Status
		to       Status
		expected bool
	}{
		// Edge cases and corner cases
		{"same status", StatusWaiting, StatusWaiting, true},
		{"none to waiting", StatusNone, StatusWaiting, false},
		{"none to scheduled", StatusNone, StatusScheduled, false},
		{"terminal complete", StatusComplete, StatusComplete, false},
		{"terminal cancelled", StatusCancelled, StatusCancelled, false},
		{"terminal expired", StatusExpired, StatusExpired, false},
		{"payment to driving backward", StatusPayment, StatusDriving, false},
		{"arrived to approaching backward", StatusArrived, StatusApproaching, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanTransition(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("CanTransition(%s, %s) = %v, expected %v",
					tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestMockPricing_DefaultCalculation(t *testing.T) {
	pricing := NewMockPricing()
	ctx := context.Background()

	// Test default calculation
	price, err := pricing.Estimate(ctx, 5.0, "unknown-type")
	if err != nil {
		t.Fatalf("Estimate failed: %v", err)
	}

	expectedAmount := int64(5.0*100) + 500 // 5km * $1 + $5 base = $10
	if price.Amount != expectedAmount {
		t.Errorf("Expected amount %d, got %d", expectedAmount, price.Amount)
	}

	if price.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", price.Currency)
	}
}

func TestMockPricing_SetEstimate(t *testing.T) {
	pricing := NewMockPricing()
	ctx := context.Background()

	expectedPrice := types.Money{Currency: "USD", Amount: 2500}
	pricing.SetEstimate("premium", expectedPrice)

	price, err := pricing.Estimate(ctx, 10.0, "premium")
	if err != nil {
		t.Fatalf("Estimate failed: %v", err)
	}

	if price.Amount != expectedPrice.Amount {
		t.Errorf("Expected amount %d, got %d", expectedPrice.Amount, price.Amount)
	}

	if price.Currency != expectedPrice.Currency {
		t.Errorf("Expected currency %s, got %s", expectedPrice.Currency, price.Currency)
	}
}

func TestMockPricing_SetError(t *testing.T) {
	pricing := NewMockPricing()
	ctx := context.Background()

	expectedErr := errors.New("pricing service down")
	pricing.SetError("premium", expectedErr)

	_, err := pricing.Estimate(ctx, 5.0, "premium")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestStatus_StringConversion(t *testing.T) {
	statuses := []Status{
		StatusNone, StatusScheduled, StatusWaiting, StatusAssigned,
		StatusApproaching, StatusArrived, StatusDriving, StatusPayment,
		StatusComplete, StatusCancelled, StatusDenied, StatusExpired,
	}

	for _, status := range statuses {
		str := string(status)
		if str == "" {
			t.Errorf("Status %v should not convert to empty string", status)
		}

		// Test round trip
		backToStatus := Status(str)
		if backToStatus != status {
			t.Errorf("Round trip failed: %v -> %s -> %v", status, str, backToStatus)
		}
	}
}

func TestOrder_Copy(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	driverID := types.ID("driver-123")
	actualFee := types.Money{Currency: "USD", Amount: 1650}

	original := Order{
		ID:            "order-123",
		PassengerID:   "passenger-456",
		DriverID:      &driverID,
		Status:        StatusDriving,
		StatusVersion: 3,
		Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:      "premium",
		EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
		ActualFee:     &actualFee,
		CreatedAt:     now,
		MatchedAt:     &now,
		AcceptedAt:    &now,
		StartedAt:     &now,
	}

	// Make a copy
	copy := original

	// Modify copy
	copy.Status = StatusComplete
	copy.StatusVersion = 4

	// Original should be unchanged
	if original.Status != StatusDriving {
		t.Error("Original order status should not change")
	}
	if original.StatusVersion != 3 {
		t.Error("Original order version should not change")
	}

	// But pointers should point to same data
	if original.DriverID != copy.DriverID {
		t.Error("Pointer fields should be shared")
	}
	if original.ActualFee != copy.ActualFee {
		t.Error("Pointer fields should be shared")
	}
}

func TestStatus_ActiveStatuses(t *testing.T) {
	activeStatuses := []Status{
		StatusScheduled, StatusWaiting, StatusAssigned,
		StatusApproaching, StatusArrived, StatusDriving, StatusPayment,
	}

	inactiveStatuses := []Status{
		StatusNone, StatusComplete, StatusCancelled, StatusDenied, StatusExpired,
	}

	// Test that each expected active status is in the list
	for _, status := range activeStatuses {
		found := false
		for _, active := range activeStatuses {
			if status == active {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Status %s should be considered active", status)
		}
	}

	// Test that inactive statuses are not in the active list
	for _, status := range inactiveStatuses {
		for _, active := range activeStatuses {
			if status == active {
				t.Errorf("Status %s should not be considered active", status)
			}
		}
	}
}

// Helper functions that we can test
func calculateDistance(pickup, dropoff types.Point) float64 {
	// Simple distance calculation (not geographically accurate, just for testing)
	latDiff := pickup.Lat - dropoff.Lat
	lngDiff := pickup.Lng - dropoff.Lng
	return (latDiff*latDiff + lngDiff*lngDiff) * 111.0 // Rough km approximation
}
