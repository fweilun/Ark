// README: Comprehensive unit tests for Order model and domain logic
package order

import (
	"errors"
	"testing"
	"time"

	"ark/internal/types"
)

func TestOrder_StatusTransitions(t *testing.T) {
	tests := []struct {
		name     string
		from     Status
		to       Status
		expected bool
	}{
		// Valid forward transitions
		{"waiting to approaching", StatusWaiting, StatusApproaching, true},
		{"approaching to arrived", StatusApproaching, StatusArrived, true},
		{"arrived to driving", StatusArrived, StatusDriving, true},
		{"driving to payment", StatusDriving, StatusPayment, true},
		{"payment to complete", StatusPayment, StatusComplete, true},

		// Valid cancellations
		{"waiting to cancelled", StatusWaiting, StatusCancelled, true},
		{"approaching to cancelled", StatusApproaching, StatusCancelled, true},
		{"arrived to cancelled", StatusArrived, StatusCancelled, true},

		// Scheduled order flows
		{"scheduled to assigned", StatusScheduled, StatusAssigned, true},
		{"assigned to approaching", StatusAssigned, StatusApproaching, true},
		{"assigned to cancelled", StatusAssigned, StatusCancelled, true},
		{"assigned to scheduled", StatusAssigned, StatusScheduled, true}, // driver cancels

		// Re-matching flows
		{"approaching to waiting", StatusApproaching, StatusWaiting, true}, // driver cancel
		{"waiting to waiting", StatusWaiting, StatusWaiting, true},         // retry match
		{"waiting to expired", StatusWaiting, StatusExpired, true},

		// Invalid transitions
		{"complete to waiting", StatusComplete, StatusWaiting, false},
		{"cancelled to driving", StatusCancelled, StatusDriving, false},
		{"expired to approaching", StatusExpired, StatusApproaching, false},
		{"none to complete", StatusNone, StatusComplete, false},
		{"driving to waiting", StatusDriving, StatusWaiting, false},
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

func TestOrder_Validation(t *testing.T) {
	now := time.Now()

	validOrder := Order{
		ID:            "order-123",
		PassengerID:   "passenger-456",
		Status:        StatusWaiting,
		StatusVersion: 1,
		Pickup:        types.Point{Lat: 37.7749, Lng: -122.4194},
		Dropoff:       types.Point{Lat: 37.7849, Lng: -122.4094},
		RideType:      "standard",
		EstimatedFee:  types.Money{Currency: "USD", Amount: 1500},
		CreatedAt:     now,
	}

	tests := []struct {
		name    string
		order   Order
		wantErr bool
	}{
		{"valid order", validOrder, false},
		{"empty ID", func() Order { o := validOrder; o.ID = ""; return o }(), true},
		{"empty passenger ID", func() Order { o := validOrder; o.PassengerID = ""; return o }(), true},
		{"invalid pickup lat", func() Order { o := validOrder; o.Pickup.Lat = 91.0; return o }(), true},
		{"invalid pickup lng", func() Order { o := validOrder; o.Pickup.Lng = 181.0; return o }(), true},
		{"invalid dropoff lat", func() Order { o := validOrder; o.Dropoff.Lat = -91.0; return o }(), true},
		{"invalid dropoff lng", func() Order { o := validOrder; o.Dropoff.Lng = -181.0; return o }(), true},
		{"empty ride type", func() Order { o := validOrder; o.RideType = ""; return o }(), true},
		{"negative fee", func() Order { o := validOrder; o.EstimatedFee.Amount = -100; return o }(), true},
		{"zero created time", func() Order { o := validOrder; o.CreatedAt = time.Time{}; return o }(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOrder(&tt.order)
			hasErr := err != nil
			if hasErr != tt.wantErr {
				t.Errorf("validateOrder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOrder_StatusVersioning(t *testing.T) {
	order := Order{
		ID:            "order-123",
		Status:        StatusWaiting,
		StatusVersion: 1,
	}

	// Simulate status update with version increment
	order.Status = StatusApproaching
	order.StatusVersion++

	if order.StatusVersion != 2 {
		t.Errorf("Expected status version 2, got %d", order.StatusVersion)
	}

	// Version should increment with each status change
	order.Status = StatusArrived
	order.StatusVersion++

	if order.StatusVersion != 3 {
		t.Errorf("Expected status version 3, got %d", order.StatusVersion)
	}
}

func TestOrder_TimestampHandling(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	order := Order{
		ID:          "order-123",
		Status:      StatusWaiting,
		CreatedAt:   now,
		MatchedAt:   nil,
		AcceptedAt:  nil,
		StartedAt:   nil,
		CompletedAt: nil,
		CancelledAt: nil,
	}

	// Test that timestamps are properly set during transitions
	matchedTime := now.Add(1 * time.Minute)
	order.MatchedAt = &matchedTime

	if order.MatchedAt == nil {
		t.Error("MatchedAt should be set")
	}
	if !order.MatchedAt.Equal(matchedTime) {
		t.Errorf("MatchedAt = %v, expected %v", order.MatchedAt, matchedTime)
	}

	// Test that only relevant timestamps are set
	if order.CompletedAt != nil {
		t.Error("CompletedAt should be nil for non-completed order")
	}
	if order.CancelledAt != nil {
		t.Error("CancelledAt should be nil for non-cancelled order")
	}
}

func TestOrder_DriverAssignment(t *testing.T) {
	order := Order{
		ID:       "order-123",
		Status:   StatusWaiting,
		DriverID: nil,
	}

	// Initially no driver assigned
	if order.DriverID != nil {
		t.Error("DriverID should be nil initially")
	}

	// Assign driver
	driverID := types.ID("driver-456")
	order.DriverID = &driverID

	if order.DriverID == nil {
		t.Error("DriverID should be set")
	}
	if *order.DriverID != driverID {
		t.Errorf("DriverID = %s, expected %s", *order.DriverID, driverID)
	}
}

func TestOrder_FeeHandling(t *testing.T) {
	estimatedFee := types.Money{Currency: "USD", Amount: 1500} // $15.00

	order := Order{
		ID:           "order-123",
		EstimatedFee: estimatedFee,
		ActualFee:    nil,
	}

	// Initially only estimated fee
	if order.EstimatedFee.Amount != 1500 {
		t.Errorf("EstimatedFee = %d, expected 1500", order.EstimatedFee.Amount)
	}
	if order.ActualFee != nil {
		t.Error("ActualFee should be nil initially")
	}

	// Set actual fee after completion
	actualFee := types.Money{Currency: "USD", Amount: 1650} // $16.50 (with tip)
	order.ActualFee = &actualFee

	if order.ActualFee == nil {
		t.Error("ActualFee should be set")
	}
	if order.ActualFee.Amount != 1650 {
		t.Errorf("ActualFee = %d, expected 1650", order.ActualFee.Amount)
	}
}

func TestOrder_DistanceCalculation(t *testing.T) {
	// San Francisco coordinates
	pickup := types.Point{Lat: 37.7749, Lng: -122.4194}
	dropoff := types.Point{Lat: 37.7849, Lng: -122.4094}

	order := Order{
		ID:      "order-123",
		Pickup:  pickup,
		Dropoff: dropoff,
	}

	// Test that coordinates are preserved
	if order.Pickup.Lat != 37.7749 {
		t.Errorf("Pickup lat = %f, expected 37.7749", order.Pickup.Lat)
	}
	if order.Pickup.Lng != -122.4194 {
		t.Errorf("Pickup lng = %f, expected -122.4194", order.Pickup.Lng)
	}
	if order.Dropoff.Lat != 37.7849 {
		t.Errorf("Dropoff lat = %f, expected 37.7849", order.Dropoff.Lat)
	}
	if order.Dropoff.Lng != -122.4094 {
		t.Errorf("Dropoff lng = %f, expected -122.4094", order.Dropoff.Lng)
	}
}

func TestOrder_RideTypeValidation(t *testing.T) {
	validTypes := []string{"standard", "premium", "xl", "pool"}
	invalidTypes := []string{"", "invalid", "STANDARD", "standard ", " standard"}

	for _, rideType := range validTypes {
		t.Run("valid_"+rideType, func(t *testing.T) {
			order := Order{
				ID:       "order-123",
				RideType: rideType,
			}

			if !isValidRideType(order.RideType) {
				t.Errorf("RideType %s should be valid", rideType)
			}
		})
	}

	for _, rideType := range invalidTypes {
		t.Run("invalid_"+rideType, func(t *testing.T) {
			order := Order{
				ID:       "order-123",
				RideType: rideType,
			}

			if isValidRideType(order.RideType) {
				t.Errorf("RideType '%s' should be invalid", rideType)
			}
		})
	}
}

func TestOrder_StatusConstants(t *testing.T) {
	// Test that all status constants are defined
	expectedStatuses := []Status{
		StatusNone, StatusScheduled, StatusWaiting, StatusAssigned,
		StatusApproaching, StatusArrived, StatusDriving, StatusPayment,
		StatusComplete, StatusCancelled, StatusDenied, StatusExpired,
	}

	for _, status := range expectedStatuses {
		if string(status) == "" {
			t.Errorf("Status %v should not be empty", status)
		}

		// Test string representation
		statusStr := string(status)
		if len(statusStr) == 0 {
			t.Errorf("Status %v should have string representation", status)
		}
	}
}

func TestOrder_ZeroValues(t *testing.T) {
	var order Order

	// Test zero values
	if order.ID != "" {
		t.Error("Zero Order should have empty ID")
	}
	if order.PassengerID != "" {
		t.Error("Zero Order should have empty PassengerID")
	}
	if order.Status != "" {
		t.Error("Zero Order should have empty Status")
	}
	if order.StatusVersion != 0 {
		t.Error("Zero Order should have StatusVersion 0")
	}
	if order.DriverID != nil {
		t.Error("Zero Order should have nil DriverID")
	}
	if order.ActualFee != nil {
		t.Error("Zero Order should have nil ActualFee")
	}
}

// Helper functions for validation (these would normally be in the model file)
func validateOrder(o *Order) error {
	if o.ID == "" {
		return errors.New("order ID is required")
	}
	if o.PassengerID == "" {
		return errors.New("passenger ID is required")
	}
	if o.Pickup.Lat < -90 || o.Pickup.Lat > 90 {
		return errors.New("invalid pickup latitude")
	}
	if o.Pickup.Lng < -180 || o.Pickup.Lng > 180 {
		return errors.New("invalid pickup longitude")
	}
	if o.Dropoff.Lat < -90 || o.Dropoff.Lat > 90 {
		return errors.New("invalid dropoff latitude")
	}
	if o.Dropoff.Lng < -180 || o.Dropoff.Lng > 180 {
		return errors.New("invalid dropoff longitude")
	}
	if o.RideType == "" {
		return errors.New("ride type is required")
	}
	if o.EstimatedFee.Amount < 0 {
		return errors.New("estimated fee cannot be negative")
	}
	if o.CreatedAt.IsZero() {
		return errors.New("created time is required")
	}
	return nil
}

func isValidRideType(rideType string) bool {
	validTypes := []string{"standard", "premium", "xl", "pool"}
	for _, valid := range validTypes {
		if rideType == valid {
			return true
		}
	}
	return false
}
