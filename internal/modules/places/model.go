package places

// Place represents a simplified location result.
type Place struct {
	Name             string
	Address          string
	Rating           float32
	PlaceID          string
	UserRatingsTotal int
}

// SearchOptions holds dynamic search refinement parameters from the AI.
type SearchOptions struct {
	// SearchKeywords are positive refinements appended to the API query (e.g. "鮮花").
	SearchKeywords string
	// ExcludeKeywords are terms that disqualify any result containing them (e.g. ["永生花", "乾燥花"]).
	ExcludeKeywords []string
}
