package location

import (
	"math"
	"testing"

	"ark/internal/types"
)

func TestHaversineKm_KnownDistances(t *testing.T) {
	tests := []struct {
		name      string
		lat1      float64
		lng1      float64
		lat2      float64
		lng2      float64
		wantKm    float64
		tolerance float64
	}{
		{
			name:      "same point",
			lat1:      25.033, lng1: 121.565,
			lat2:      25.033, lng2: 121.565,
			wantKm:    0,
			tolerance: 0.001,
		},
		{
			name:      "Taipei 101 to Taipei Main Station (~3km)",
			lat1:      25.0340, lng1: 121.5645,
			lat2:      25.0478, lng2: 121.5170,
			wantKm:    5.2,
			tolerance: 1.0,
		},
		{
			name:      "New York to Los Angeles (~3944km)",
			lat1:      40.7128, lng1: -74.0060,
			lat2:      34.0522, lng2: -118.2437,
			wantKm:    3944,
			tolerance: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := haversineKm(tt.lat1, tt.lng1, tt.lat2, tt.lng2)
			if math.Abs(got-tt.wantKm) > tt.tolerance {
				t.Errorf("haversineKm() = %f, want %f (±%f)", got, tt.wantKm, tt.tolerance)
			}
		})
	}
}

func TestHaversineKm_Symmetry(t *testing.T) {
	d1 := haversineKm(25.0, 121.0, 26.0, 122.0)
	d2 := haversineKm(26.0, 122.0, 25.0, 121.0)
	if math.Abs(d1-d2) > 0.0001 {
		t.Errorf("haversine is not symmetric: %f vs %f", d1, d2)
	}
}

func TestSortByDistance_Drivers(t *testing.T) {
	drivers := []DriverLocation{
		{DriverID: types.ID("c"), Distance: 5.0},
		{DriverID: types.ID("a"), Distance: 1.0},
		{DriverID: types.ID("b"), Distance: 3.0},
	}

	sortByDistance(drivers, func(d DriverLocation) float64 { return d.Distance })

	if drivers[0].DriverID != "a" || drivers[1].DriverID != "b" || drivers[2].DriverID != "c" {
		t.Errorf("unexpected sort order: %v", drivers)
	}
}

func TestSortByDistance_Passengers(t *testing.T) {
	passengers := []PassengerLocation{
		{PassengerID: types.ID("z"), Distance: 10.0},
		{PassengerID: types.ID("x"), Distance: 2.0},
		{PassengerID: types.ID("y"), Distance: 5.0},
	}

	sortByDistance(passengers, func(p PassengerLocation) float64 { return p.Distance })

	if passengers[0].PassengerID != "x" || passengers[1].PassengerID != "y" || passengers[2].PassengerID != "z" {
		t.Errorf("unexpected sort order: %v", passengers)
	}
}

func TestSortByDistance_Empty(t *testing.T) {
	var drivers []DriverLocation
	sortByDistance(drivers, func(d DriverLocation) float64 { return d.Distance })
}

func TestSortByDistance_Single(t *testing.T) {
	drivers := []DriverLocation{
		{DriverID: types.ID("a"), Distance: 2.0},
	}
	sortByDistance(drivers, func(d DriverLocation) float64 { return d.Distance })
	if drivers[0].DriverID != "a" {
		t.Errorf("single element sort failed")
	}
}
