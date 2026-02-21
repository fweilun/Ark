package ai

// IntentResult captures the structured output from the AI model.
type IntentResult struct {
	// Intent describes the user's primary goal.
	// Valid values: "booking", "clarification", "chat", "completed".
	Intent string `json:"intent"`

	// Destination is the target location extracted from the user's input.
	Destination *string `json:"destination,omitempty"`

	// StartLocation is the starting point of the trip.
	// Defaults to "Current Location" if not specified.
	StartLocation *string `json:"start_location,omitempty"`

	// NeedsOrigin indicates if the AI needs to ask for the starting location.
	NeedsOrigin *bool `json:"needs_origin,omitempty"`

	// NeedsSearch indicates if the user wants to stop by somewhere (e.g. buy flowers).
	NeedsSearch *bool `json:"needs_search,omitempty"`

	// SearchCategory describes what the user wants to search for (e.g. "florist", "coffee").
	SearchCategory *string `json:"search_category,omitempty"`

	// SearchKeywords holds positive refinement keywords the user specified
	// (e.g. "鮮花", "有機"). These are appended to the Places API query string.
	SearchKeywords *string `json:"search_keywords,omitempty"`

	// ExcludeKeywords holds terms the user explicitly wants to exclude from results
	// (e.g. ["永生花", "乾燥花"]). Any place whose name contains one of these is filtered out.
	ExcludeKeywords []string `json:"exclude_keywords,omitempty"`

	// IntermediateStop is the name of the selected stop (e.g. "flower shop A").
	IntermediateStop *string `json:"intermediate_stop,omitempty"`

	// TimeType indicates the nature of the time constraint.
	// Valid values: "arrival_time", "pickup_time", or null.
	TimeType *string `json:"time_type,omitempty"`

	// ISOTime is the absolute timestamp (YYYY-MM-DDTHH:mm:ss) calculated based
	// on the user's relative input and the current context.
	ISOTime *string `json:"iso_time,omitempty"`

	// PassengerCount is the number of passengers. Defaults to 1 if not mentioned.
	PassengerCount int `json:"passenger_count,omitempty"`

	// HasPet indicates if the user mentioned bringing a pet (cat, dog, etc.).
	HasPet bool `json:"has_pet,omitempty"`

	// SelectedUpgrade is the car type the user chose during upsell (e.g. "豪華速速", "寵物專車").
	// Empty string means the user declined the upgrade.
	SelectedUpgrade string `json:"selected_upgrade,omitempty"`

	// Reply is the response to show to the user.
	// If clarification is needed, it asks a specific question.
	Reply string `json:"reply"`
}
