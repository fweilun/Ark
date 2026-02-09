package pricing

import (
	"context"
	"testing"
	"time"
)

func TestService_Estimate(t *testing.T) {
	// Base time: 2026-02-10 12:00:00 (Normal day, Off-peak)
	baseTime := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
    
    // Peak time: 08:00
    peakTime := time.Date(2026, 2, 10, 8, 0, 0, 0, time.UTC)
    
    // Night time: 23:30
    nightTime := time.Date(2026, 2, 10, 23, 30, 0, 0, time.UTC)
    
    // Spring Festival: 2026-02-17
    festiveTime := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		req      PricingRequest
		wantFare int64
	}{
		{
			name: "Base Fare Only (< 1.25km, 0 min)",
			req: PricingRequest{
				DistanceKm:  1.0,
				DurationMin: 0,
				RequestTime: baseTime,
				Weather:     "normal",
				CarType:     "normal",
			},
			wantFare: 85,
		},
		{
			name: "Distance Charge (1.65km -> 0.4km excess -> 2 units * $5)",
			req: PricingRequest{
				DistanceKm:  1.65,
				DurationMin: 0,
				RequestTime: baseTime,
			},
			wantFare: 85 + 10, // 95
		},
		{
			name: "Time Charge Off-Peak (10 min -> $30)",
			req: PricingRequest{
				DistanceKm:  1.0,
				DurationMin: 10,
				RequestTime: baseTime,
			},
			wantFare: 85 + 30, // 115
		},
		{
			name: "Time Charge Peak (10 min -> $50)",
			req: PricingRequest{
				DistanceKm:  1.0,
				DurationMin: 10,
				RequestTime: peakTime,
			},
			wantFare: 85 + 50, // 135
		},
		{
			name: "Distance Adjustment 5-6km (-$2/min off-peak -> $1/min)",
			req: PricingRequest{
				DistanceKm:  5.5,
				DurationMin: 10,
				RequestTime: baseTime,
			},
			// Base: 85
			// Dist: 5.5 - 1.25 = 4.25. ceil(4.25/0.2) = 22. 22*5 = 110.
			// Time: 10 * (3-2) = 10.
			// Total: 85 + 110 + 10 = 205.
			wantFare: 205,
		},
		{
			name: "Distance Adjustment >7km (+$2/min off-peak -> $5/min)",
			req: PricingRequest{
				DistanceKm:  10.0,
				DurationMin: 10,
				RequestTime: baseTime,
			},
			// Base: 85
			// Dist: 10 - 1.25 = 8.75. ceil(8.75/0.2) = 44. 44*5 = 220.
			// Time: 10 * (3+2) = 50.
			// Total: 85 + 220 + 50 = 355.
			wantFare: 355, 
		},
		{
			name: "Night Surcharge (+$25)",
			req: PricingRequest{
				DistanceKm:  1.0,
				RequestTime: nightTime,
			},
			wantFare: 85 + 25, // 110
		},
		{
			name: "Spring Festival Surcharge (+$40)",
			req: PricingRequest{
				DistanceKm:  1.0,
				RequestTime: festiveTime,
			},
			wantFare: 85 + 40, // 125
		},
		{
			name: "Weather Rain (x1.15)",
			req: PricingRequest{
				DistanceKm:  1.0,
				RequestTime: baseTime,
				Weather:     "rain",
			},
			// 85 * 1.15 = 97.75 -> 98
			wantFare: 98,
		},
		{
			name: "Car Type Lucky Cat (x1.5)",
			req: PricingRequest{
				DistanceKm:  1.0,
				RequestTime: baseTime,
				CarType:     "lucky_cat",
			},
			// 85 * 1.5 = 127.5 -> 128
			wantFare: 128,
		},
        {
            name: "Complex Combination",
            req: PricingRequest{
                DistanceKm: 8.0, // >7km -> Time adjustment +2
                DurationMin: 20,
                RequestTime: peakTime, // Peak -> $5/min. Total rate: 5+2=7.
                Weather: "heavy_rain", // x1.3
                CarType: "lucky_cat", // x1.5
            },
            // Base: 85
            // Dist: 8.0 - 1.25 = 6.75. ceil(6.75/0.2) = 34. 34*5 = 170.
            // Time: 20 * (5+2) = 140.
            // Surcharge: 0 (Day, Non-festive).
            // Subtotal: 85 + 170 + 140 = 395.
            // Multiplier: 395 * 1.3 * 1.5 = 395 * 1.95 = 770.25 -> 771.
            wantFare: 771,
        },
	}

	s := NewService(nil) // Store not needed for Estimate logic

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Estimate(context.Background(), tt.req)
			if err != nil {
				t.Errorf("Estimate() error = %v", err)
				return
			}
			if got.TotalAmount != tt.wantFare {
				t.Errorf("Estimate() = %v, want %v", got.TotalAmount, tt.wantFare)
			}
		})
	}
}
