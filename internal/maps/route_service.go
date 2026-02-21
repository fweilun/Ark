package maps

import (
	"context"
	"fmt"
	"time"

	"googlemaps.github.io/maps"
)

// RouteService handles interactions with Google Maps API.
type RouteService struct {
	client *maps.Client
}

// NewRouteService creates a new RouteService with the given API Key.
func NewRouteService(apiKey string) (*RouteService, error) {
	client, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create maps client: %w", err)
	}
	return &RouteService{client: client}, nil
}

// GetTravelEstimate returns the duration and distance string for a trip from origin to destination.
// It assumes driving mode.
func (s *RouteService) GetTravelEstimate(ctx context.Context, origin, destination string) (time.Duration, string, error) {
	r := &maps.DirectionsRequest{
		Origin:      origin,
		Destination: destination,
		Mode:        maps.TravelModeDriving,
		Language:    "zh-TW", // Traditional Chinese for consistency
		Region:      "TW",    // Bias results to Taiwan
	}

	routes, _, err := s.client.Directions(ctx, r)
	if err != nil {
		return 0, "", fmt.Errorf("maps api error: %w", err)
	}

	if len(routes) == 0 || len(routes[0].Legs) == 0 {
		return 0, "", fmt.Errorf("no route found")
	}

	leg := routes[0].Legs[0]
	return leg.Duration, leg.Distance.HumanReadable, nil
}
