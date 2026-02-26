package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"ark/internal/integration"
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
	aiProvider       aiusage.AIClient
	routeService     *maps.RouteService
	placesService    *maps.PlacesService
	loc              *time.Location
	userName         string
	userPhone        string
	HasDiningIntent  bool
	TargetRestaurant string
	DiningPrompted   bool
	LastDestination  string
	RideFullyBooked  bool
	CurrentState     string
}

// NewTripPlanner creates a TripPlanner with initialized dependencies.
func NewTripPlanner(aiProvider aiusage.AIClient, routeService *maps.RouteService, placesService *maps.PlacesService, userName string, userPhone string) (*TripPlanner, error) {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		return nil, fmt.Errorf("failed to load Asia/Taipei location: %w", err)
	}
	return &TripPlanner{
		aiProvider:    aiProvider,
		routeService:  routeService,
		placesService: placesService,
		loc:           loc,
		userName:      userName,
		userPhone:     userPhone,
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
		return "\n\n已自動為您安排 **" + carType + "**。"
	default:
		return "\n\n請問需要為您升級為《豪華速速》，或是特殊車輛（如寵物、大容量）嗎？"
	}
}

// PlanTrip processes a user message and returns a conversational response with trip details.
func (p *TripPlanner) PlanTrip(ctx context.Context, userMessage string, userLocation string, userContextInfo string) (string, error) {
	// 1. Prepare Context for AI
	now := time.Now().In(p.loc)
	currentContext := map[string]string{
		"current_time":      now.Format(time.RFC3339),
		"user_location":     userLocation,
		"user_context_info": userContextInfo,
		"user_name":         p.userName,
		"has_dining_intent": fmt.Sprintf("%v", p.HasDiningIntent),
		"target_restaurant": p.TargetRestaurant,
		"dining_prompted":   fmt.Sprintf("%v", p.DiningPrompted),
		"destination":       p.LastDestination,
		"ride_fully_booked": fmt.Sprintf("%v", p.RideFullyBooked),
	}

	// 2. Call AI to Parse Intent
	intent, err := p.aiProvider.ParseUserIntent(ctx, userMessage, currentContext)
	if err != nil {
		log.Printf("AI Error: %v", err)
		return "", fmt.Errorf("ai error: %w", err)
	}

	// 2.1 TRULY PERSISTENT STATE RECOVERY (State Latching)
	if intent.IsDiningIntent {
		p.HasDiningIntent = true
	}
	if intent.RestaurantName != "" {
		p.TargetRestaurant = intent.RestaurantName
	}
	if intent.Destination != nil && *intent.Destination != "" {
		p.LastDestination = *intent.Destination
	}

	// If persistent state is active, check if we need to clear it or apply it
	if p.HasDiningIntent {
		// Only the LLM can clear the state by outputting IsDiningIntent = false when the user explicitly declines (Rule 14),
		// but since the LLM sometimes drops it by mistake during upsell turns, we will forcibly restore it
		// UNLESS the user explicitly mentioned restaurant decline keywords.
		userDeclinedDining := strings.Contains(userMessage, "不用餐廳") || strings.Contains(userMessage, "自己訂") || strings.Contains(userMessage, "不用定位") || strings.Contains(userMessage, "不用訂位")

		if userDeclinedDining {
			// Explicit decline of dining prompt: Clear persistent state
			p.HasDiningIntent = false
			p.TargetRestaurant = ""
			intent.IsDiningIntent = false
			intent.NeedsReservation = false
		} else {
			// Safely restore state to current intent if LLM forgot it during an upsell turn
			intent.IsDiningIntent = true
			if intent.RestaurantName == "" {
				intent.RestaurantName = p.TargetRestaurant
			}
		}
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

	// 2.7 徹底隔離管家狀態的輸出邏輯 (State Isolation)
	if p.CurrentState == "dining_concierge" {
		// 1. 情況 A：需要觸發目的地餐廳推薦
		if intent.NeedsDestinationSearch || (intent.NeedsSearch != nil && *intent.NeedsSearch && intent.RestaurantName == "") {
			if p.LastDestination != "" {
				places, err := p.placesService.SearchAtDestination(ctx, p.LastDestination, "餐廳")
				if err != nil || len(places) == 0 {
					return fmt.Sprintf("不好意思，我在 %s 附近暫時找不到適合的熱門餐廳。", p.LastDestination), nil
				}

				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("為您推薦 %s 附近評價最高的幾間餐廳：\n\n", p.LastDestination))
				for i, place := range places {
					sb.WriteString(fmt.Sprintf("%d. **%s** (%.1f ⭐️, %d 則評論)\n", i+1, place.Name, place.Rating, place.UserRatingsTotal))
					sb.WriteString(fmt.Sprintf("   地址：%s\n", place.Address))
				}
				sb.WriteString("\n請問有喜歡哪一間嗎？需要幫您用預設資訊訂位嗎？")
				return sb.String(), nil
			}
			return intent.Reply, nil
		}

		// 2. 情況 C：【核心修復】確認訂位
		// 只有在使用者明確答覆人數並且同意後，這個 flag 才會是 true
		if intent.NeedsReservation {
			var t time.Time
			if intent.ISOTime != nil {
				t, _ = time.Parse(time.RFC3339, *intent.ISOTime)
			}
			dest := p.LastDestination
			rName := intent.RestaurantName
			if rName == "" {
				rName = dest
			}
			timeStr := "抵達時"
			if !t.IsZero() {
				weekdayMap := map[time.Weekday]string{
					time.Sunday: "週日", time.Monday: "週一", time.Tuesday: "週二",
					time.Wednesday: "週三", time.Thursday: "週四", time.Friday: "週五",
					time.Saturday: "週六",
				}
				timeStr = fmt.Sprintf("%s (%s) %s", t.Format("1/02"), weekdayMap[t.Weekday()], t.Format("15:04"))
			}
			inlineMsg := integration.BookInline(rName, timeStr, p.userName, p.userPhone, intent.PassengerCount)

			p.HasDiningIntent = false
			p.CurrentState = ""
			return inlineMsg + "\n\n請於用餐時間準時到場，祝您用餐愉快！", nil
		}

		// 3. 【防吞噬防線】情況 B / 其他所有情況
		// 直接無條件回傳 LLM 思考出來的 Reply 字串！(例如它在問人數、推薦等)
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

		if intent.IsDiningIntent || category == "restaurant" || category == "餐廳" {
			places, err = p.placesService.SearchAtDestination(ctx, dest, category)
		} else {
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
			detour := time.Duration(0)
			isDining := intent.IsDiningIntent || category == "restaurant" || category == "餐廳"
			if !isDining {
				var err error
				detour, err = p.routeService.GetDetourEstimate(ctx, origin, place.Address, dest, targetTime)
				if err != nil || detour.Minutes() > 15 {
					continue // Skip if calc fails or detour too long
				}
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
			if intent.IsDiningIntent || category == "restaurant" || category == "餐廳" {
				detourStr = "目的地附近"
			}
			suggestions = append(suggestions, fmt.Sprintf("[%s] (⭐%.1f, %s)", rec.Place.Name, rec.Place.Rating, detourStr))
		}

		reason := fmt.Sprintf("最建議選擇 [%s]", bestOption.Place.Name)
		if !intent.IsDiningIntent && category != "restaurant" && category != "餐廳" {
			reason += fmt.Sprintf("，因為它的繞路時間最短（僅 %.0f 分鐘）", bestOption.Detour.Minutes())
		}
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

	// 3.5 Handle COMPLETED intent (upsell resolved, or reservation declined)
	if intent.Intent == "completed" {
		var confirmMsg string
		if intent.SelectedUpgrade == "" {
			confirmMsg = "沒問題，已為您維持一般車型。"
		} else {
			confirmMsg = fmt.Sprintf("沒問題，已為您升級為【%s】。", intent.SelectedUpgrade)
		}

		// 步驟 A (訂位完成)
		if intent.NeedsReservation {
			var t time.Time
			if intent.ISOTime != nil {
				t, _ = time.Parse(time.RFC3339, *intent.ISOTime)
			}
			dest := p.LastDestination
			rName := intent.RestaurantName
			if rName == "" {
				rName = dest
			}
			timeStr := "抵達時"
			if !t.IsZero() {
				weekdayMap := map[time.Weekday]string{
					time.Sunday: "週日", time.Monday: "週一", time.Tuesday: "週二",
					time.Wednesday: "週三", time.Thursday: "週四", time.Friday: "週五",
					time.Saturday: "週六",
				}
				timeStr = fmt.Sprintf("%s (%s) %s", t.Format("1/02"), weekdayMap[t.Weekday()], t.Format("15:04"))
			}
			inlineMsg := integration.BookInline(rName, timeStr, p.userName, p.userPhone, intent.PassengerCount)

			// Mark dining flow as entirely resolved since it's booked
			p.HasDiningIntent = false
			return confirmMsg + "\n\n" + inlineMsg + "\n\n行程已全數確認，司機將準時為您服務！祝您行程順利 🚗", nil
		}

		// 步驟 B (管家發問)
		if p.HasDiningIntent && !p.DiningPrompted {
			// 【關鍵】攔截！切換到管家狀態，防止對話結束
			p.DiningPrompted = true
			p.RideFullyBooked = true // Start the Concierge Dining phase
			p.CurrentState = "dining_concierge"

			var finalReply string
			if p.TargetRestaurant != "" && p.TargetRestaurant != p.LastDestination {
				// 情境 A: 已經知道餐廳
				finalReply = fmt.Sprintf("👉 偵測到您準備前往 %s，需要幫您用預設資訊（%s）透過 Inline 預約嗎？", p.TargetRestaurant, p.userName)
			} else {
				// 情境 B: 還不知道餐廳
				finalReply = "👉 另外發現您打算用餐，有需要幫您預約哪間餐廳嗎？還是需要為您推薦目的地附近的熱門餐廳？"
			}

			// 直接回傳我們組合好的管家台詞，捨棄 LLM 原本說的廢話
			return confirmMsg + "\n\n" + finalReply, nil
		}

		// 步驟 C (真正結案)
		return confirmMsg + "\n\n行程已全數確認，司機將準時為您服務！祝您行程順利 🚗", nil
	}

	// 3.6 Handle Non-Booking Intents or Clarification
	if intent.Intent == "clarification" || intent.Intent == "chat" {
		reply := intent.Reply

		// If clarification resulted in a reservation, execute Step A here as well.
		// (This covers the scenario where the LLM used "clarification" instead of "completed" to book)
		if intent.NeedsReservation {
			var t time.Time
			if intent.ISOTime != nil {
				t, _ = time.Parse(time.RFC3339, *intent.ISOTime)
			}
			dest := p.LastDestination
			rName := intent.RestaurantName
			if rName == "" {
				rName = dest
			}
			timeStr := "抵達時"
			if !t.IsZero() {
				weekdayMap := map[time.Weekday]string{
					time.Sunday: "週日", time.Monday: "週一", time.Tuesday: "週二",
					time.Wednesday: "週三", time.Thursday: "週四", time.Friday: "週五",
					time.Saturday: "週六",
				}
				timeStr = fmt.Sprintf("%s (%s) %s", t.Format("1/02"), weekdayMap[t.Weekday()], t.Format("15:04"))
			}
			inlineMsg := integration.BookInline(rName, timeStr, p.userName, p.userPhone, intent.PassengerCount)

			p.HasDiningIntent = false
			reply = inlineMsg + "\n\n" + reply
			if !strings.Contains(reply, "祝您行程順利") {
				reply += "\n\n行程已全數確認，司機將準時為您服務！祝您行程順利 🚗"
			}
			return reply, nil
		}

		return reply, nil
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
			msg := fmt.Sprintf("%s收到！已為您在行程中加入 **%s** 作為中途停靠站。\n為確保您能在 **%s** 抵達 %s，出發時間將提前至 **%s**。%s",
				warning, waypoint,
				fmtWithWeekday(targetTime), destination,
				fmtWithWeekday(departureTime),
				carTypeFooter(carType, specialNotice))
			return p.appendDiningPrompt(intent, msg, targetTime, destination), nil
		}

		// Forward scheduling or no time specified.
		departureTime := now
		if !targetTime.IsZero() && intent.TimeType != nil && *intent.TimeType == "pickup_time" {
			departureTime = targetTime
		}
		estimatedArrival := departureTime.Add(totalDuration)
		msg := fmt.Sprintf("收到！已為您在行程中加入 **%s** 作為中途停靠站。\n將於 **%s** 從 %s 出發，中途停靠 **%s**，預計於 **%s** 抵達 %s。%s",
			waypoint,
			fmtWithWeekday(departureTime), origin, waypoint,
			fmtWithWeekday(estimatedArrival), destination,
			carTypeFooter(carType, specialNotice))
		return p.appendDiningPrompt(intent, msg, targetTime, destination), nil
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

		return p.appendDiningPrompt(intent, responseMsg, targetTime, destination), nil
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

// appendDiningPrompt appends post-booking dining or reservation text to the base response message.
// It intercepts "booking" / "needs_reservation" signals and appends or prepends Inline mock strings.
func (p *TripPlanner) appendDiningPrompt(intent *aiusage.IntentResult, baseMsg string, targetTime time.Time, dest string) string {
	if intent.NeedsReservation {
		// Mock booking confirmation
		timeStr := "抵達時"
		if !targetTime.IsZero() {
			weekdayMap := map[time.Weekday]string{
				time.Sunday: "週日", time.Monday: "週一", time.Tuesday: "週二",
				time.Wednesday: "週三", time.Thursday: "週四", time.Friday: "週五",
				time.Saturday: "週六",
			}
			timeStr = fmt.Sprintf("%s (%s) %s", targetTime.Format("1/02"), weekdayMap[targetTime.Weekday()], targetTime.Format("15:04"))
		}

		rName := intent.RestaurantName
		if rName == "" {
			rName = dest
		}

		inlineMsg := integration.BookInline(rName, timeStr, p.userName, p.userPhone, intent.PassengerCount)
		// Booking confirmation takes visual priority, but both belong in response.
		return inlineMsg + "\n\n" + baseMsg
	}

	if !intent.IsDiningIntent {
		return baseMsg
	}

	if intent.RestaurantName != "" {
		return baseMsg + fmt.Sprintf("\n\n偵測到您將前往 %s，請問需要幫您以預設資訊（%s / %s）透過 Inline 進行訂位嗎？", intent.RestaurantName, p.userName, maskedPhone(p.userPhone))
	}

	return baseMsg + "\n\n另外發現您打算用餐，請問已選好餐廳了嗎？還是需要為您推薦目的地附近的熱門餐廳？"
}

// maskedPhone formats the phone number to hide the middle digits.
func maskedPhone(phone string) string {
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, " ", "")
	if len(phone) >= 10 {
		return phone[:4] + "-xxx-" + phone[len(phone)-3:]
	}
	return phone
}
