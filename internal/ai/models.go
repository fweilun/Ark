package ai

// IntentResult captures the structured output from the AI model.
type IntentResult struct {
	// Intent describes the user's primary goal (e.g., "booking", "chat", "route_planning").
	Intent string `json:"intent"`

	// Destination is the target location extracted from the user's input.
	// Nullable because not all intents have a destination (e.g. "chat").
	Destination *string `json:"destination,omitempty"`

	// TimeType indicates the nature of the time constraint.
	// Valid values: "arrival_time" (deadline), "pickup_time" (start), "immediate".
	TimeType string `json:"time_type"`

	// ISOTime is the absolute timestamp (YYYY-MM-DDTHH:mm:ss) calculated based
	// on the user's relative input and the current context.
	ISOTime *string `json:"iso_time,omitempty"`

	// PassengerNote contains any extra context or requests from the user.
	PassengerNote string `json:"passenger_note,omitempty"`

	// Reply is a short, polite response to the user, acting as the "Taiwanese Ride Dispatcher".
	Reply string `json:"reply"`
}
