package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"ark/internal/maps"
	"ark/internal/modules/aiusage"
)

// DefaultTrafficBuffer is the extra time added to ensure on-time arrival.
const DefaultTrafficBuffer = 10 * time.Minute

// isTimePrecise checks that the iso_time string contains an explicit hour from the
// user (i.e., not midnight 00:00 which is the AI's typical auto-fill default).
// Returns false if the string is empty, unparseable, or falls exactly on midnight
// without any plausible user intent (midnight bookings are vanishingly rare).
// The backend treats midnight as a strong signal that the AI guessed the time.
func isTimePrecise(isoTime *string) bool {
	if isoTime == nil || *isoTime == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, *isoTime)
	if err != nil {
		// Malformed — not precise
		return false
	}
	// If the time component is exactly 00:00:00, treat it as an AI-guessed placeholder.
	// Real late-night requests (e.g., midnight) are extremely rare and should be confirmed anyway.
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
		return false
	}
	return true
}

// TripPlanner orchestrates the AI intent parsing and Google Maps routing.
type TripPlanner struct {
	aiProvider    aiusage.AIClient
	routeService  *maps.RouteService
	placesService *maps.PlacesService
	loc           *time.Location
}

// NewTripPlanner creates a TripPlanner with initialized dependencies.
func NewTripPlanner(aiProvider aiusage.AIClient, routeService *maps.RouteService, placesService *maps.PlacesService) (*TripPlanner, error) {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		return nil, fmt.Errorf("failed to load Asia/Taipei location: %w", err)
	}
	return &TripPlanner{
		aiProvider:    aiProvider,
		routeService:  routeService,
		placesService: placesService,
		loc:           loc,
	}, nil
}

// resolveCarType determines the appropriate car type and any special notice
// based on passenger count and pet status extracted from the AI intent.
func resolveCarType(passengerCount int, hasPet bool) (carType string, specialNotice string) {
	switch {
	case hasPet:
		return "寵物專車", ""
	case passengerCount >= 5 && passengerCount <= 6:
		return "六人座大車", ""
	case passengerCount > 6:
		return "一般車型", "❗ 由於人數超過 6 人，請問需要為您安排多輛車嗎？"
	default:
		return "一般車型", ""
	}
}

// carTypeFooter builds the third section of the booking response.
func carTypeFooter(carType, specialNotice string) string {
	switch {
	case specialNotice != "":
		return "\n\n⚠️ " + specialNotice
	case carType == "寵物專車" || carType == "六人座大車":
		return fmt.Sprintf("\n\n已自動為您安排 **%s**。", carType)
	default:
		return "\n\n請問需要為您升級為《豪華速速》，或是特殊車輛（如寵物、大容量）嗎？"
	}
}

// PlanTrip processes a user message and returns a conversational response with trip details.
func (p *TripPlanner) PlanTrip(ctx context.Context, userMessage string, userLocation string, userContextInfo string) (string, error) {
	// 1. Prepare Context for AI
	// Ensure we use Taipei time for the "current time" context passed to AI
	now := time.Now().In(p.loc)
	currentContext := map[string]string{
		"current_time":      now.Format(time.RFC3339),
		"user_location":     userLocation,
		"user_context_info": userContextInfo,
	}

	// 2. Call AI to Parse Intent
	intent, err := p.aiProvider.ParseUserIntent(ctx, userMessage, currentContext)
	if err != nil {
		log.Printf("AI Error: %v", err)
		return "", fmt.Errorf("ai error: %w", err)
	}

	// 2.5 Determine Origin (Required for Search and Booking)
	origin := userLocation
	if intent.StartLocation != nil && *intent.StartLocation != "" && *intent.StartLocation != "Current Location" {
		origin = *intent.StartLocation
	}

	// Alias Resolution (Demo)
	switch origin {
	case "Home", "家", "我家":
		origin = "社子街3號"
	case "Company", "公司":
		origin = "台北101"
	}

	// 2.6 Check if Origin is Needed (PRIORITY: Before Search)
	if intent.NeedsOrigin != nil && *intent.NeedsOrigin {
		return intent.Reply, nil
	}

	// 3. Handle Search Intent (V2) - Intercept before Clarification check
	if intent.NeedsSearch != nil && *intent.NeedsSearch {
		// ── BACKEND ORIGIN GUARD ─────────────────────────────────────────
		// Refuse to search if we don't have a meaningful start location.
		// "Current Location" without real coordinates is not usable for route-based search.
		originIsKnown := origin != "" && origin != "Current Location" && origin != "UNKNOWN_LOCATION"
		if !originIsKnown {
			return "收到您的需求！請問您預計從哪裡出發，以便為您尋找順路的地點？", nil
		}

		// ── BACKEND TIME PRECISION GUARD ─────────────────────────────────
		// The AI must never reach the search branch with a vague / auto-filled time.
		// If the iso_time looks like a midnight placeholder, demand a concrete hour.
		if !isTimePrecise(intent.ISOTime) {
			log.Printf("[TimePolicer] Search blocked: iso_time is absent or midnight placeholder (%v)", intent.ISOTime)
			return "系統需要您提供精確的時間（例如晚上 7 點、20:00）才能為您進行後續規劃。請問您具體幾點出發或抵達？", nil
		}

		category := "something"
		if intent.SearchCategory != nil {
			category = *intent.SearchCategory
		}

		// Semantic Polish
		if category == "florist" {
			category = "花店"
		}

		// Search Along Route (V3)
		var places []maps.Place
		dest := "Typical Destination" // Fallback
		if intent.Destination != nil && *intent.Destination != "" {
			dest = *intent.Destination
		}

		// Build search options from AI-parsed keywords.
		searchOpts := &maps.SearchOptions{}
		if intent.SearchKeywords != nil && *intent.SearchKeywords != "" {
			searchOpts.SearchKeywords = *intent.SearchKeywords
		}
		if len(intent.ExcludeKeywords) > 0 {
			searchOpts.ExcludeKeywords = intent.ExcludeKeywords
		}

		// 1. Get Route Waypoints (Origin -> Mid -> Dest)
		waypoints, err := p.routeService.GetRouteWaypoints(ctx, origin, dest)
		if err != nil {
			log.Printf("Route Waypoints Error: %v", err)
			// Fallback to simple search near origin if route fails
			places, err = p.placesService.SearchNearby(ctx, origin, category, searchOpts)
		} else {
			// 2. Search at Waypoints
			places, err = p.placesService.SearchAlongRoute(ctx, waypoints, category, searchOpts)
		}

		if err != nil {
			log.Printf("Places Search Error: %v", err)
			return fmt.Sprintf("抱歉，搜尋 %s 時發生錯誤，請稍後再試。", category), nil
		}

		if len(places) == 0 {
			return fmt.Sprintf("抱歉，沿著去 %s 的路徑上找不到合適的 %s。", dest, category), nil
		}

		// Parse Target Time for Feasibility Check
		var targetTime time.Time
		if intent.ISOTime != nil {
			if t, err := time.Parse(time.RFC3339, *intent.ISOTime); err == nil {
				targetTime = t.In(p.loc)

				// Date Expiry Check (Search Flow)
				if targetTime.Before(now) {
					diff := now.Sub(targetTime)
					if diff < 12*time.Hour {
						tomorrow := targetTime.Add(24 * time.Hour)
						return fmt.Sprintf("由於現在時間已晚，請問您是指 **明天 (%s)** %s 抵達 %s 嗎？",
							tomorrow.Format("1/02"), targetTime.Format("15:04"), dest), nil
					}
				}
			}
		}

		// Direct Duration (needed for total time calc and search-path time warning).
		// Pass targetTime so Maps uses departure-time traffic, not current traffic.
		directDur, _, _ := p.routeService.GetTravelEstimate(ctx, origin, dest, targetTime)
		activityBuffer := 10 * time.Minute

		// Recommendation Struct for Sorting
		type Recommendation struct {
			Place  maps.Place
			Detour time.Duration
		}
		var recommendations []Recommendation

		// Calculate Detours & Filter
		for _, place := range places {
			detour, err := p.routeService.GetDetourEstimate(ctx, origin, place.Address, dest, targetTime)
			if err != nil {
				continue // Skip if calc fails
			}

			// Filter: Detour > 15 mins is too much
			if detour.Minutes() > 15 {
				continue
			}

			recommendations = append(recommendations, Recommendation{
				Place:  place,
				Detour: detour,
			})
		}

		if len(recommendations) == 0 {
			return fmt.Sprintf("抱歉，雖然找到了 %s，但在順路 15 分鐘範圍內沒有合適的選擇。", category), nil
		}

		// Sort Logic: Detour Ascending, Tie-breaker Rating Descending
		// Simple bubble sort or similar for small list (max ~3-5 from Places API?)
		// Actually maps returns up to 20, we limited to top 3 in search?
		// SearchAlongRoute returns broader list. Let's sort all valid ones.
		// Go 1.21+ has slices.SortFunc, but let's stick to simple logic or sort.Slice
		// We can't import "sort" easily without messing imports?
		// Let's just use a simple bubble sort for this small list.
		for i := 0; i < len(recommendations)-1; i++ {
			for j := 0; j < len(recommendations)-i-1; j++ {
				a := recommendations[j]
				b := recommendations[j+1]

				swap := false
				detourDiff := a.Detour.Minutes() - b.Detour.Minutes()

				// Primary: Detour Ascending
				if detourDiff > 2.0 { // A is significantly longer -> Swap
					swap = true
				} else if detourDiff < -2.0 { // A is significantly shorter -> Keep
					swap = false
				} else {
					// Tie-breaker: Rating Descending
					if b.Place.Rating > a.Place.Rating {
						swap = true
					}
				}

				if swap {
					recommendations[j], recommendations[j+1] = recommendations[j+1], recommendations[j]
				}
			}
		}

		// Pick Top 3
		topN := 3
		if len(recommendations) < topN {
			topN = len(recommendations)
		}

		bestOption := recommendations[0]

		// Detailed Warnings (Reality Check - Strict Pre-calculation)
		var warningMsg string
		if !targetTime.IsZero() && intent.TimeType != nil && *intent.TimeType == "arrival_time" {
			totalNeeded := directDur + bestOption.Detour + activityBuffer
			projectedArrival := now.Add(totalNeeded)

			if projectedArrival.After(targetTime) {
				delay := projectedArrival.Sub(targetTime)
				// Strict Format: "⚠️ 提醒：現在是 HH:MM，前往 [Dest] 已相當緊迫..."
				warningMsg = fmt.Sprintf("⚠️ 提醒：現在是 %s，前往 %s 已相當緊迫。若加上 %s 行程，預計抵達時間為 %s，將延遲 %.0f 分鐘。\n\n",
					now.Format("15:04"), dest, category, projectedArrival.Format("15:04"), delay.Minutes())
			}
		}

		// Build Response
		var suggestions []string
		for i := 0; i < topN; i++ {
			rec := recommendations[i]
			detourStr := fmt.Sprintf("繞路 %.0f 分鐘", rec.Detour.Minutes())
			suggestions = append(suggestions, fmt.Sprintf("[%s] (⭐%.1f, %s)", rec.Place.Name, rec.Place.Rating, detourStr))
		}

		reason := fmt.Sprintf("最建議選擇 [%s]，因為它的繞路時間最短（僅 %.0f 分鐘）", bestOption.Place.Name, bestOption.Detour.Minutes())
		if bestOption.Place.Rating >= 4.8 {
			reason += fmt.Sprintf("，且維持 %.1f 的高評分", bestOption.Place.Rating)
		}
		reason += "。"

		// Build acknowledgement prefix if user specified refinements.
		var ackPrefix string
		if len(intent.ExcludeKeywords) > 0 || (intent.SearchKeywords != nil && *intent.SearchKeywords != "") {
			parts := []string{}
			if len(intent.ExcludeKeywords) > 0 {
				parts = append(parts, fmt.Sprintf("已排除『%s』", strings.Join(intent.ExcludeKeywords, "』『")))
			}
			if intent.SearchKeywords != nil && *intent.SearchKeywords != "" {
				parts = append(parts, fmt.Sprintf("專門尋找『%s』", *intent.SearchKeywords))
			}
			ackPrefix = strings.Join(parts, "，") + "。\n\n"
		}

		optionsMsg := fmt.Sprintf("%s%s偵測到您想找%s。沿著去%s的路徑，我為您找到了以下順路的選擇：\n\n%s\n\n%s確認後請告訴我，將為您更新預約時間。",
			warningMsg,
			ackPrefix,
			category,
			dest,
			func() string {
				res := ""
				for _, s := range suggestions {
					res += s + "\n\n"
				}
				return res
			}(),
			reason)

		return optionsMsg, nil
	}

	// 3.5 Handle COMPLETED intent (upsell resolved — end of conversation)
	if intent.Intent == "completed" {
		if intent.SelectedUpgrade == "" {
			// User declined upgrade
			return "沒問題，已維持一般車型。行程已全數確認，司機將準時為您服務！祝您行程順利 🚗", nil
		}
		// User accepted / named a specific upgrade
		return fmt.Sprintf("沒問題，已為您升級為【%s】。行程已全數確認，司機將準時為您服務！祝您行程順利 🚗", intent.SelectedUpgrade), nil
	}

	// 3.6 Handle Non-Booking Intents or Clarification
	if intent.Intent == "clarification" || intent.Intent == "chat" {
		return intent.Reply, nil
	}

	// 3.7 Guard: Booking with incomplete time must be re-clarified.
	// The AI should never reach "booking" without a concrete iso_time, but this
	// is a defence-in-depth layer in case the prompt rule slips through.
	if intent.Intent == "booking" && (intent.ISOTime == nil || *intent.ISOTime == "") {
		reply := intent.Reply
		if reply == "" {
			reply = "請問您希望幾點出發（或抵達）呢？"
		}
		return reply, nil
	}

	// 4. Handle Missing Destination (Safety Check)
	if intent.Destination == nil || *intent.Destination == "" {
		return intent.Reply, nil
	}
	destination := *intent.Destination

	// 4.5 BACKEND BOOKING TIME PRECISION GUARD
	// Before committing to full route calculation, ensure the booking carries a
	// real user-given hour. If the AI guessed midnight as a placeholder, stop here.
	// Only apply when a non-immediate time type is set (pure "immediate" has no iso_time anyway).
	timeTypeIsSet := intent.TimeType != nil && *intent.TimeType != "immediate" && *intent.TimeType != ""
	if timeTypeIsSet && !isTimePrecise(intent.ISOTime) {
		log.Printf("[TimePolicer] Booking blocked: iso_time is absent or midnight placeholder (%v)", intent.ISOTime)
		return "系統需要您提供精確的時間（例如晚上 7 點、20:00）才能進行預約。請問您具體幾點出發或抵達？", nil
	}

	// Alias Resolution (Demo)
	switch destination {
	case "Home", "家", "回家", "home":
		destination = "社子街3號"
	case "Company", "公司":
		destination = "台北101"
	}

	// 6. Calculate Ride via Maps API (Standard Booking)
	// IF IntermediateStop is present, we calculate the full chain
	var totalDuration time.Duration
	var responseMsg string

	// Format times for display (M/DD (ChineseWeekday) HH:mm)
	// e.g., "2/17 (週二) 22:00"
	weekdayMap := map[time.Weekday]string{
		time.Sunday:    "週日",
		time.Monday:    "週一",
		time.Tuesday:   "週二",
		time.Wednesday: "週三",
		time.Thursday:  "週四",
		time.Friday:    "週五",
		time.Saturday:  "週六",
	}
	fmtWithWeekday := func(t time.Time) string {
		return fmt.Sprintf("%s (%s) %s", t.Format("1/02"), weekdayMap[t.Weekday()], t.Format("15:04"))
	}

	// 6a. EXPLICIT WAYPOINTS — user named specific places; no detour filter, direct route.
	if len(intent.ExplicitWaypoints) > 0 {
		// Use first waypoint (multi-waypoint support can extend this loop).
		waypoint := intent.ExplicitWaypoints[0]

		// Leg 1: Origin → Waypoint
		leg1, _, err := p.routeService.GetTravelEstimate(ctx, origin, waypoint, time.Time{})
		if err != nil {
			return "", fmt.Errorf("explicit waypoint leg1: %w", err)
		}
		// Leg 2: Waypoint → Destination
		leg2, _, err := p.routeService.GetTravelEstimate(ctx, waypoint, destination, time.Time{})
		if err != nil {
			return "", fmt.Errorf("explicit waypoint leg2: %w", err)
		}
		activity := 5 * time.Minute // Brief stop allowance
		totalDuration := leg1 + activity + leg2

		carType, specialNotice := resolveCarType(intent.PassengerCount, intent.HasPet)

		var targetTime time.Time
		if intent.ISOTime != nil {
			if t, err := time.Parse(time.RFC3339, *intent.ISOTime); err == nil {
				targetTime = t.In(p.loc)
			}
		}

		isArrival := intent.TimeType != nil && *intent.TimeType == "arrival_time"

		if isArrival && !targetTime.IsZero() {
			// Reverse scheduling with traffic buffer (same as standard booking path).
			departureTime := targetTime.Add(-totalDuration).Add(-DefaultTrafficBuffer)
			var warning string
			if departureTime.Before(now) {
				delay := now.Add(totalDuration).Sub(targetTime)
				warning = fmt.Sprintf("⚠️ 提醣：建議出發時間已過（%s），若現在立刻出發，預計將延遲 %.0f 分鐘抵達。\n\n",
					fmtWithWeekday(departureTime), delay.Minutes())
				departureTime = now
			}
			return fmt.Sprintf("%s收到！已為您在行程中加入 **%s** 作為中途停靠站。\n為確保您能在 **%s** 抵達 %s，出發時間將提前至 **%s**。%s",
				warning, waypoint,
				fmtWithWeekday(targetTime), destination,
				fmtWithWeekday(departureTime),
				carTypeFooter(carType, specialNotice)), nil
		}

		// Forward scheduling or no time specified.
		departureTime := now
		if !targetTime.IsZero() && intent.TimeType != nil && *intent.TimeType == "pickup_time" {
			departureTime = targetTime
		}
		estimatedArrival := departureTime.Add(totalDuration)
		return fmt.Sprintf("收到！已為您在行程中加入 **%s** 作為中途停靠站。\n將於 **%s** 從 %s 出發，中途停靠 **%s**，預計於 **%s** 抵達 %s。%s",
			waypoint,
			fmtWithWeekday(departureTime), origin, waypoint,
			fmtWithWeekday(estimatedArrival), destination,
			carTypeFooter(carType, specialNotice)), nil
	}

	// 6b. INTERMEDIATE STOP (from search selection / confirmed stop via AI):
	if intent.IntermediateStop != nil && *intent.IntermediateStop != "" {
		stop := *intent.IntermediateStop

		// Parse target time first so we can pass it to Maps for traffic-aware estimates.
		var targetTime time.Time
		if intent.ISOTime != nil {
			if t, err := time.Parse(time.RFC3339, *intent.ISOTime); err == nil {
				targetTime = t.In(p.loc)
			}
		}

		// Leg 1: Origin -> Stop
		leg1, _, err := p.routeService.GetTravelEstimate(ctx, origin, stop, targetTime)
		if err != nil {
			return "", fmt.Errorf("failed to calc leg1: %w", err)
		}

		// Activity at stop
		activity := 10 * time.Minute

		// Leg 2: Stop -> Dest
		leg2, _, err := p.routeService.GetTravelEstimate(ctx, stop, destination, targetTime)
		if err != nil {
			return "", fmt.Errorf("failed to calc leg2: %w", err)
		}

		totalDuration = leg1 + activity + leg2

		// Determine vehicle type.
		carType, specialNotice := resolveCarType(intent.PassengerCount, intent.HasPet)

		isArrivalTime := intent.TimeType != nil && *intent.TimeType == "arrival_time"
		var departureTime, requiredArrivalTime time.Time

		if isArrivalTime && !targetTime.IsZero() {
			// ── REVERSE SCHEDULING ──────────────────────────────────────────
			requiredArrivalTime = targetTime
			departureTime = targetTime.Add(-totalDuration)

			// Sanity check: is it already too late to depart?
			if departureTime.Before(now) {
				delay := now.Add(totalDuration).Sub(targetTime)
				var warningMsg string
				if delay > 0 {
					warningMsg = fmt.Sprintf("⚠️ 提醒：現在是 %s，距離 %s 抵達 %s 的時間已不足。 若現在立刻出發，預計將延遲 %.0f 分鐘抵達。\n\n",
						now.Format("15:04"), fmtWithWeekday(targetTime), destination, delay.Minutes())
					departureTime = now
					requiredArrivalTime = now.Add(totalDuration)
				}
				responseMsg = fmt.Sprintf("%s收到！已幫您預約叫車。\n已煤安排從 %s 出發，中途停靠 **%s**，預計於 **%s** 抵達 %s（延遲 %.0f 分鐘）。%s",
					warningMsg, origin, stop,
					fmtWithWeekday(requiredArrivalTime), destination, delay.Minutes(),
					carTypeFooter(carType, specialNotice))
			} else {
				// On time — show the reverse-calculated departure.
				responseMsg = fmt.Sprintf("收到！已幫您預約叫車。\n為了讓您在 **%s** 準時抵達 %s，將於 **%s** 從 %s 出發，中途停靠 **%s**（約 10 分鐘）。%s",
					fmtWithWeekday(requiredArrivalTime), destination,
					fmtWithWeekday(departureTime), origin, stop,
					carTypeFooter(carType, specialNotice))
			}
		} else {
			// ── FORWARD SCHEDULING ──────────────────────────────────────────
			if !targetTime.IsZero() {
				departureTime = targetTime
			} else {
				departureTime = now
			}
			requiredArrivalTime = departureTime.Add(totalDuration)

			responseMsg = fmt.Sprintf("收到！已幫您預約叫車。\n將於 **%s** 從 %s 出發，中途停靠 **%s**（約 10 分鐘），預計於 **%s** 抵達 %s。%s",
				fmtWithWeekday(departureTime), origin, stop,
				fmtWithWeekday(requiredArrivalTime), destination,
				carTypeFooter(carType, specialNotice))
		}

		return responseMsg, nil
	}

	// Standard Direct Trip
	// Standard Direct Trip: parse target time first for traffic-aware estimate.
	var targetTime time.Time
	if intent.ISOTime != nil {
		parsedTime, err := time.Parse(time.RFC3339, *intent.ISOTime)
		if err == nil {
			targetTime = parsedTime.In(p.loc)
			// Date Expiry Check: if within 12 hours past, suggest tomorrow.
			if targetTime.Before(now) {
				diff := now.Sub(targetTime)
				if diff < 12*time.Hour {
					tomorrow := targetTime.Add(24 * time.Hour)
					return fmt.Sprintf("由於現在時間已晚 (%s)，請問您是指 **明天 (%s)** %s 抵達 %s 嗎？",
						now.Format("15:04"), tomorrow.Format("1/02"), targetTime.Format("15:04"), destination), nil
				}
			}
		} else {
			log.Printf("Time Parse Error: %v (input: %s)", err, *intent.ISOTime)
		}
	}

	duration, _, err := p.routeService.GetTravelEstimate(ctx, origin, destination, targetTime)

	// If no specific time logic or "immediate", handle simple case
	if intent.TimeType == nil || *intent.TimeType == "immediate" || targetTime.IsZero() {
		// Just provide estimate
		return fmt.Sprintf("收到！從%s去%s車程約 %.0f 分鐘。現在幫您叫車嗎？", origin, destination, duration.Minutes()), nil
	}

	timeType := *intent.TimeType

	// Determine vehicle type for this booking.
	carType, specialNotice := resolveCarType(intent.PassengerCount, intent.HasPet)

	if timeType == "arrival_time" {
		// Reverse scheduling: user's target arrival is the anchor.
		suggestedPickup := targetTime.Add(-duration).Add(-DefaultTrafficBuffer)

		// Sanity check: is the suggested pickup time already past?
		var extraWarning string
		if suggestedPickup.Before(now) {
			delay := now.Add(duration).Sub(targetTime)
			extraWarning = fmt.Sprintf("⚠️ 提醒：建議出發時間已過（%s），若現在立刻出發，預計將延遲 %.0f 分鐘抵達。\n\n",
				fmtWithWeekday(suggestedPickup), delay.Minutes())
			suggestedPickup = now
		}

		responseMsg = fmt.Sprintf("%s收到！已幫您預約叫車。\n為了讓您在 **%s** 準時抵達 %s，將於 **%s** 從 %s 出發。預計車程 %.0f 分鐘。%s",
			extraWarning,
			fmtWithWeekday(targetTime), destination,
			fmtWithWeekday(suggestedPickup), origin,
			duration.Minutes(),
			carTypeFooter(carType, specialNotice))
	} else if timeType == "pickup_time" {
		// Forward scheduling: user picked a departure time.
		estimatedArrival := targetTime.Add(duration)
		responseMsg = fmt.Sprintf("收到！已幫您預約叫車。\n將於 **%s** 從 %s 出發前往 %s。預計車程 %.0f 分鐘，於 **%s** 抵達。%s",
			fmtWithWeekday(targetTime), origin, destination,
			duration.Minutes(), fmtWithWeekday(estimatedArrival),
			carTypeFooter(carType, specialNotice))
	} else {
		// Fallback — no time info.
		responseMsg = fmt.Sprintf("收到！已幫您預約叫車。從 %s 去 %s 車程約 %.0f 分鐘。%s",
			origin, destination, duration.Minutes(),
			carTypeFooter(carType, specialNotice))
	}

	return responseMsg, nil
}
