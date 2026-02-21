package maps

import (
	"context"
	"fmt"
	"strings"

	"googlemaps.github.io/maps"
)

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

// PlacesService handles interactions with Google Places API.
type PlacesService struct {
	client *maps.Client
}

// NewPlacesService creates a new PlacesService with the given API Key.
func NewPlacesService(apiKey string) (*PlacesService, error) {
	client, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create maps client: %w", err)
	}
	return &PlacesService{client: client}, nil
}

// SearchNearby searches for places matching the query near the given location.
// opts can be nil for a basic search. SearchKeywords are appended to the query;
// ExcludeKeywords filter out results by name.
func (s *PlacesService) SearchNearby(ctx context.Context, location string, query string, opts *SearchOptions) ([]Place, error) {
	// Build the query string.
	fullQuery := query

	// Append positive keywords to the query for a more specific API request.
	if opts != nil && opts.SearchKeywords != "" {
		fullQuery = opts.SearchKeywords + " " + fullQuery
	}

	if location != "" && location != "Current Location" {
		fullQuery = fmt.Sprintf("%s near %s", fullQuery, location)
	}

	// Determine if this is a florist search to apply strict type filtering.
	isFloristSearch := strings.Contains(strings.ToLower(query), "flower") ||
		strings.Contains(strings.ToLower(query), "florist") ||
		strings.Contains(query, "花")

	r := &maps.TextSearchRequest{
		Query:    fullQuery,
		OpenNow:  true, // Filter for open places
		Language: "zh-TW",
		Region:   "TW",
	}

	// Strict type filter for florist searches to prevent irrelevant results from the API.
	if isFloristSearch {
		r.Type = "florist"
	}

	resp, err := s.client.TextSearch(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("places api error: %w", err)
	}

	// Base exclusion list: always filter out parks, dessert shops, etc.
	excludedKeywords := []string{"Park", "公園", "Douhua", "豆花", "Shaved Ice", "剉冰", "Soup", "羹", "Tofu", "豆腐"}

	// Extra exclusion for florists to avoid supermarkets and convenience stores.
	// Applied as a second line of defence after the API type filter.
	if isFloristSearch {
		excludedKeywords = append(excludedKeywords,
			"Supermarket", "Mart", "Carrefour", "PX Mart",
			"Convenience Store", "Seven", "Family", "Hi-Life", "Ok Mart",
			"全聯", "家樂福", "超市", "便利商店", "超商", "萊爾富", "全家", "統一")
	}

	// Merge in dynamic exclusions from the AI (e.g. "永生花", "乾燥花").
	if opts != nil && len(opts.ExcludeKeywords) > 0 {
		excludedKeywords = append(excludedKeywords, opts.ExcludeKeywords...)
	}

	var results []Place

	for _, result := range resp.Results {
		if result.Rating < 4.0 { // Filter for high quality
			continue
		}

		// Name filtering: static + dynamic exclusions.
		skip := false
		for _, kw := range excludedKeywords {
			if kw != "" && strings.Contains(result.Name, kw) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		results = append(results, Place{
			Name:             result.Name,
			Address:          result.FormattedAddress,
			Rating:           result.Rating,
			PlaceID:          result.PlaceID,
			UserRatingsTotal: result.UserRatingsTotal,
		})

		if len(results) >= 3 { // Limit to top 3
			break
		}
	}

	return results, nil
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// SearchAlongRoute searches for places near multiple waypoints along a route.
func (s *PlacesService) SearchAlongRoute(ctx context.Context, waypoints []string, query string, opts *SearchOptions) ([]Place, error) {
	uniquePlaces := make(map[string]Place)
	var allPlaces []Place

	for _, point := range waypoints {
		results, err := s.SearchNearby(ctx, point, query, opts)
		if err != nil {
			continue // Skip failed points, try others
		}

		for _, p := range results {
			if _, exists := uniquePlaces[p.PlaceID]; !exists {
				uniquePlaces[p.PlaceID] = p
				allPlaces = append(allPlaces, p)
			}
		}
	}

	if len(allPlaces) == 0 {
		return nil, nil // No results found
	}

	return allPlaces, nil
}
