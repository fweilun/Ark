// README: Pricing rate definition for each ride type.
package pricing


import "time"

type Rate struct {
    RideType string
    BaseFare int64
    PerKm    int64
    Currency string
}

type PricingRequest struct {
    DistanceKm  float64
    DurationMin float64
    RequestTime time.Time
    Weather     string // "rain", "heavy_rain", "normal"
    CarType     string // "lucky_cat", "normal"
    Tolls       float64
}

type PricingResult struct {
    TotalAmount   int64
    DriverShare   int64
    PlatformShare int64
    Currency      string
    Breakdown     map[string]int64
}
