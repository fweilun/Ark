package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"ark/internal/ai"
	"ark/internal/maps"
)

// DefaultTrafficBuffer is the extra time added to ensure on-time arrival.
const DefaultTrafficBuffer = 10 * time.Minute

// TripPlanner orchestrates the AI intent parsing and Google Maps routing.
type TripPlanner struct {
	aiProvider   *ai.GeminiProvider
	routeService *maps.RouteService
	loc          *time.Location
}

// NewTripPlanner creates a TripPlanner with initialized dependencies.
func NewTripPlanner(aiProvider *ai.GeminiProvider, routeService *maps.RouteService) (*TripPlanner, error) {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		return nil, fmt.Errorf("failed to load Asia/Taipei location: %w", err)
	}
	return &TripPlanner{
		aiProvider:   aiProvider,
		routeService: routeService,
		loc:          loc,
	}, nil
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

	// 3. Handle Non-Booking Intents or Clarification
	if intent.Intent == "clarification" || intent.Intent == "chat" {
		return intent.Reply, nil
	}

	// 3.5 Check if Origin is Needed
	if intent.NeedsOrigin != nil && *intent.NeedsOrigin {
		// AI determined it needs origin and should have asked in Reply.
		return intent.Reply, nil
	}

	// 4. Handle Missing Destination (Safety Check)
	if intent.Destination == nil || *intent.Destination == "" {
		return intent.Reply, nil // Or specific error message? Let's just return reply.
	}
	destination := *intent.Destination

	// 5. Determine Origin (AI-detected or Context Default)
	origin := userLocation
	if intent.StartLocation != nil && *intent.StartLocation != "" && *intent.StartLocation != "Current Location" {
		origin = *intent.StartLocation
	}

	// 6. Calculate Ride via Maps API
	duration, _, err := p.routeService.GetTravelEstimate(ctx, origin, destination)
	if err != nil {
		log.Printf("Maps Error: %v", err)
		return "", fmt.Errorf("maps error: %w", err)
	}

	// 6. Time Math (The "ZooZoo" Magic)
	// Parse the AI's ISOTime if available
	var targetTime time.Time
	if intent.ISOTime != nil {
		// AI returns ISO 8601 with timezone (RFC3339).
		parsedTime, err := time.Parse(time.RFC3339, *intent.ISOTime)
		if err == nil {
			targetTime = parsedTime.In(p.loc)
		} else {
			log.Printf("Time Parse Error: %v (input: %s)", err, *intent.ISOTime)
			// Fallback: try parsing without offset if AI messes up, OR just log error.
			// For now, let's assume valid RFC3339 from Gemini.
		}
	}

	// If no specific time logic or "immediate", handle simple case
	if intent.TimeType == nil || *intent.TimeType == "immediate" || targetTime.IsZero() {
		// Just provide estimate
		return fmt.Sprintf("收到！從%s去%s車程約 %.0f 分鐘。現在幫您叫車嗎？", origin, destination, duration.Minutes()), nil
	}

	timeType := *intent.TimeType
	var responseMsg string

	// Format times for display (YYYY/MM/DD HH:mm)
	timeFmt := "2006/01/02 15:04"

	if timeType == "arrival_time" {
		// SuggestedPickup = TargetTime - TravelDuration - TrafficBuffer
		suggestedPickup := targetTime.Add(-duration).Add(-DefaultTrafficBuffer)
		responseMsg = fmt.Sprintf("收到！已幫您預約。為了讓您能在 %s 抵達，將安排 %s 從 %s 出發前往 %s (預計車程 %.0f 分鐘)。",
			targetTime.Format(timeFmt), suggestedPickup.Format(timeFmt), origin, destination, duration.Minutes())
	} else if timeType == "pickup_time" {
		// EstimatedArrival = TargetTime + TravelDuration
		estimatedArrival := targetTime.Add(duration)
		responseMsg = fmt.Sprintf("收到！已幫您預約。%s 從 %s 出發前往 %s，車程約 %.0f 分鐘，預計 %s 抵達。",
			targetTime.Format(timeFmt), origin, destination, duration.Minutes(), estimatedArrival.Format(timeFmt))
	} else {
		// Fallback
		responseMsg = fmt.Sprintf("收到！從%s去%s車程約 %.0f 分鐘。", origin, destination, duration.Minutes())
	}

	return responseMsg, nil
}
