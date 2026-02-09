// README: Pricing service computes fare estimates.
package pricing



import (
    "context"
    "math"
    "time"
)

type Service struct {
    store *Store
}

func NewService(store *Store) *Service {
    return &Service{store: store}
}


func (s *Service) Estimate(ctx context.Context, req PricingRequest) (PricingResult, error) {
    // 1. Base Fare
    baseFare := int64(85)
    
    // 2. Distance Charge
    distanceCharge := int64(0)
    if req.DistanceKm > 1.25 {
        excessKm := req.DistanceKm - 1.25
        // $5 per 200m (0.2km)
        // ceil(excess / 0.2) * 5
        units := math.Ceil(excessKm / 0.2)
        distanceCharge = int64(units) * 5
    }

    // 3. Time Charge
    // Peak: 07:00-09:00, 16:30-19:00 -> $5.0
    // Off-peak: $3.0
    hour := req.RequestTime.Hour()
    minute := req.RequestTime.Minute()
    totalMinutes := hour*60 + minute
    
    isPeak := false
    // 07:00 - 09:00 (420 - 540)
    if totalMinutes >= 7*60 && totalMinutes < 9*60 {
        isPeak = true
    }
    // 16:30 - 19:00 (990 - 1140)
    if totalMinutes >= 16*60+30 && totalMinutes < 19*60 {
        isPeak = true
    }

    timeRate := 3.0
    if isPeak {
        timeRate = 5.0
    }

    // Distance Adjustment for Time Charge
    // 5-6km: -$2 (min $1?) -> req says "becomes $3 or $1".
    // $5 -> $3, $3 -> $1. So it is strictly -$2.
    // >7km: +$2.
    if req.DistanceKm >= 5.0 && req.DistanceKm <= 6.0 {
        timeRate -= 2.0
    } else if req.DistanceKm > 7.0 {
        timeRate += 2.0
    }
    
    // Ensure timeRate is not negative (though logic implies min is 1.0)
    if timeRate < 0 {
        timeRate = 0
    }

    timeCharge := int64(math.Ceil(req.DurationMin * timeRate))

    // 4. Surcharges
    // Night: 23:00 - 06:00
    // RequestTime is local? Assuming consistent timezone handling (server time).
    // 23:00 (1380) - 24:00 (1440) or 00:00 - 06:00 (360)
    nightSurcharge := int64(0)
    if totalMinutes >= 23*60 || totalMinutes < 6*60 {
        nightSurcharge = 25
    }

    // Spring Festival: 2026/02/16 - 2026/02/22
    festiveSurcharge := int64(0)
    y, m, d := req.RequestTime.Date()
    if y == 2026 && m == time.February && d >= 16 && d <= 22 {
        festiveSurcharge = 40
    }

    // 5. Multipliers
    // Weather: Rain x1.15, Heavy Rain x1.3
    weatherMultiplier := 1.0
    if req.Weather == "rain" {
        weatherMultiplier = 1.15
    } else if req.Weather == "heavy_rain" || req.Weather == "heavy rain" {
        weatherMultiplier = 1.3
    }
    
    // Car Type: Lucky Cat x1.5
    carMultiplier := 1.0
    if req.CarType == "lucky_cat" || req.CarType == "lucky cat" {
        carMultiplier = 1.5
    }

    // Calculate Total
    subtotal := float64(baseFare + distanceCharge + timeCharge + nightSurcharge + festiveSurcharge)
    total := subtotal * weatherMultiplier * carMultiplier
    

    // Ceiling
    netFare := int64(math.Ceil(total))
    
    // Tolls logic
    tolls := int64(math.Ceil(req.Tolls))
    totalAmount := netFare + tolls
    
    // Split
    // Driver = (Total - Tolls) * 0.8 + Tolls = NetFare * 0.8 + Tolls
    driverShare := int64(math.Round(float64(netFare) * 0.8)) + tolls
    
    // Platform = (Total - Tolls) * 0.2 = NetFare * 0.2
    platformShare := int64(math.Round(float64(netFare) * 0.2))
    
    // Adjustment to ensure shares sum to total (give remainder to platform or driver? usually platform takes hit or driver takes remainder)
    // Let's ensure Driver + Platform == TotalAmount
    if driverShare + platformShare != totalAmount {
        // e.g. 100 split 80/20 matches.
        // 101 split 80.8/20.2 -> 81/20 = 101. Matches.
        // 102 split 81.6/20.4 -> 82/20 = 102. Matches.
        // 103 split 82.4/20.6 -> 82/21 = 103. Matches.
        // 104 split 83.2/20.8 -> 83/21 = 104. Matches.
        // math.Round goes to nearest even? No, standard round up >= 0.5.
        // Check case 1: Net = 1. 0.8 + 0.2. Round(0.8)=1, Round(0.2)=0. Sum=1.
        // Check case 2: Net = 4. 3.2 + 0.8. Round(3.2)=3, Round(0.8)=1. Sum=4.
        // Check case 3: Net = 3. 2.4 + 0.6. Round(2.4)=2, Round(0.6)=1. Sum=3.
        // It seems consistent if standard rounding. 
        // Go's math.Round: rounds to nearest integer, rounding half away from zero.
    }

    result := PricingResult{
        TotalAmount:   totalAmount,
        DriverShare:   driverShare,
        PlatformShare: platformShare,
        Currency:      "TWD",
        Breakdown: map[string]int64{
            "base_fare":         baseFare,
            "distance_charge":   distanceCharge,
            "time_charge":       timeCharge,
            "night_surcharge":   nightSurcharge,
            "festive_surcharge": festiveSurcharge,
            "tolls":             tolls,
        },
    }

    return result, nil
}
