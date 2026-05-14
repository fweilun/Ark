package aiusage

import "errors"

// ErrInsufficientTokens is returned when a user has no tokens remaining for the current month.
var ErrInsufficientTokens = errors.New("insufficient tokens")

// DefaultTokens is the number of tokens granted per month.
const DefaultTokens = 100

// ScheduleItem is a V4 composite itinerary block that bundles ride logistics
// with the corresponding on-site activity, so the frontend can render a
// unified calendar card that shows both the car leg and the venue.
type ScheduleItem struct {
	// Overall time window for this block (ride start → activity end).
	TotalStartTime string `json:"total_start_time"` // HH:mm
	TotalEndTime   string `json:"total_end_time"`   // HH:mm

	// Activity details
	ActivityTitle    string `json:"activity_title"`
	ActivityLocation string `json:"activity_location"`
	ActivityDesc     string `json:"activity_desc"`

	// Ride details
	NeedsRide         bool     `json:"needs_ride"`
	RideStartTime     string   `json:"ride_start_time"` // HH:mm
	RideEndTime       string   `json:"ride_end_time"`   // HH:mm
	RideOrigin        string   `json:"ride_origin"`
	RideDestination   string   `json:"ride_destination"`
	IntermediateStops []string `json:"intermediate_stops"` // optional waypoints
}

// IntentResult captures the structured output from the AI model.
type IntentResult struct {
	// Intent describes the user's primary goal.
	// Valid values: "booking", "clarification", "chat", "completed", "itinerary_planning".
	Intent string `json:"intent"`

	// Destination is the target location extracted from the user's input.
	Destination *string `json:"destination,omitempty"`

	// StartLocation is the starting point of the trip.
	StartLocation *string `json:"start_location,omitempty"`

	// NeedsOrigin indicates if the AI needs to ask for the starting location.
	NeedsOrigin *bool `json:"needs_origin,omitempty"`

	// NeedsSearch indicates if the user wants to stop by somewhere (e.g. buy flowers).
	NeedsSearch *bool `json:"needs_search,omitempty"`

	// SearchCategory describes what the user wants to search for (e.g. "florist", "coffee").
	SearchCategory *string `json:"search_category,omitempty"`

	// SearchKeywords holds positive refinement keywords the user specified.
	SearchKeywords *string `json:"search_keywords,omitempty"`

	// ExcludeKeywords holds terms the user explicitly wants to exclude from results.
	ExcludeKeywords []string `json:"exclude_keywords,omitempty"`

	// IntermediateStop is the name of the selected stop.
	IntermediateStop *string `json:"intermediate_stop,omitempty"`

	// TimeType indicates the nature of the time constraint.
	// Valid values: "arrival_time", "pickup_time", or null.
	TimeType *string `json:"time_type,omitempty"`

	// ISOTime is the absolute timestamp in RFC3339 format.
	ISOTime *string `json:"iso_time,omitempty"`

	// PassengerCount is the number of passengers. Defaults to 1 if not mentioned.
	PassengerCount int `json:"passenger_count,omitempty"`

	// HasPet indicates if the user mentioned bringing a pet.
	HasPet bool `json:"has_pet,omitempty"`

	// SelectedUpgrade is the car type chosen during upsell. Empty = declined.
	SelectedUpgrade string `json:"selected_upgrade,omitempty"`

	// AutoSelectStop indicates if the AI should automatically pick the best stop.
	AutoSelectStop bool `json:"auto_select_stop,omitempty"`

	// ExplicitWaypoints lists specific named places the user said to stop at en-route.
	ExplicitWaypoints []string `json:"explicit_waypoints,omitempty"`

	// IsDiningIntent is true when the user mentions dining/eating, or the destination is a restaurant.
	IsDiningIntent bool `json:"is_dining_intent,omitempty"`

	// RestaurantName is the specific restaurant name if the user has already chosen one.
	RestaurantName string `json:"restaurant_name,omitempty"`

	// NeedsReservation is true when the user has confirmed they want the system to
	// make a restaurant reservation on their behalf via Inline.
	NeedsReservation bool `json:"needs_reservation,omitempty"`

	// NeedsDestinationSearch is true when the user requests restaurant recommendations.
	NeedsDestinationSearch bool `json:"needs_destination_search,omitempty"`

	// ── V4 Itinerary Planning ────────────────────────────────────────────

	// Itinerary holds the V4 composite schedule blocks.
	Itinerary []ScheduleItem `json:"itinerary,omitempty"`

	// NeedsCharter indicates transportation preference for the itinerary.
	// nil = unanswered, true = full-day charter, false = individual hails.
	NeedsCharter *bool `json:"needs_charter,omitempty"`

	// Reply is the user-facing response string.
	Reply string `json:"reply"`
}
