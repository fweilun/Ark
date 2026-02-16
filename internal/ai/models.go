package ai

// IntentResult captures the structured output from the AI model.
type IntentResult struct {
	// Intent describes the user's primary goal.
	// Valid values: "booking", "clarification", "chat".
	Intent string `json:"intent"`

	// Destination is the target location extracted from the user's input.
	Destination *string `json:"destination,omitempty"`

	// StartLocation is the starting point of the trip.
	// Defaults to "Current Location" if not specified.
	StartLocation *string `json:"start_location,omitempty"`

	// NeedsOrigin indicates if the AI needs to ask for the starting location.
	NeedsOrigin *bool `json:"needs_origin,omitempty"`

	// TimeType indicates the nature of the time constraint.
	// Valid values: "arrival_time", "pickup_time", or null.
	TimeType *string `json:"time_type,omitempty"`

	// ISOTime is the absolute timestamp (YYYY-MM-DDTHH:mm:ss) calculated based
	// on the user's relative input and the current context.
	ISOTime *string `json:"iso_time,omitempty"`

	// Reply is the response to show to the user.
	// If clarification is needed, it asks a specific question.
	Reply string `json:"reply"`
}
