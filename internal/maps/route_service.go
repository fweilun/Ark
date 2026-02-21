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

// Geocode converts an address string to a lat,lng string.
func (s *RouteService) Geocode(ctx context.Context, address string) (string, error) {
	r := &maps.GeocodingRequest{
		Address:  address,
		Language: "zh-TW",
		Region:   "TW",
	}

	results, err := s.client.Geocode(ctx, r)
	if err != nil {
		return "", fmt.Errorf("geocoding error: %w", err)
	}

	if len(results) == 0 {
		return "", fmt.Errorf("address not found: %s", address)
	}

	loc := results[0].Geometry.Location
	return fmt.Sprintf("%f,%f", loc.Lat, loc.Lng), nil
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

	// Fallback: If no route found, try Geocoding the endpoints first
	if err != nil || len(routes) == 0 {
		// Try Geocoding
		geoOrigin, err1 := s.Geocode(ctx, origin)
		geoDest, err2 := s.Geocode(ctx, destination)

		if err1 == nil && err2 == nil {
			// Retry with Coordinates
			r.Origin = geoOrigin
			r.Destination = geoDest
			routes, _, err = s.client.Directions(ctx, r)
		}
	}

	if err != nil {
		return 0, "", fmt.Errorf("maps api error: %w", err)
	}

	if len(routes) == 0 || len(routes[0].Legs) == 0 {
		return 0, "", fmt.Errorf("no route found")
	}

	leg := routes[0].Legs[0]
	return leg.Duration, leg.Distance.HumanReadable, nil
}

// GetDetourEstimate calculates the extra time needed to add a stop.
// Returns (TP_Stop - TP_Direct).
func (s *RouteService) GetDetourEstimate(ctx context.Context, origin, stop, destination string) (time.Duration, error) {
	// 1. Direct Duration
	directDur, _, err := s.GetTravelEstimate(ctx, origin, destination)
	if err != nil {
		return 0, fmt.Errorf("failed to get direct route: %w", err)
	}

	// 2. Route with Stop
	r := &maps.DirectionsRequest{
		Origin:      origin,
		Destination: destination,
		Waypoints:   []string{stop}, // Add the stop
		Mode:        maps.TravelModeDriving,
		Language:    "zh-TW",
		Region:      "TW",
	}

	routes, _, err := s.client.Directions(ctx, r)
	if err != nil {
		return 0, fmt.Errorf("failed to get detour route: %w", err)
	}

	if len(routes) == 0 || len(routes[0].Legs) == 0 {
		return 0, fmt.Errorf("no detour route found")
	}

	// With 1 waypoint, there should be 2 legs? Or does Directions API return total duration in routes[0].Legs if optimized?
	// Actually, routes[0].Legs contains legs. We sum them up.
	var totalDuration time.Duration
	for _, leg := range routes[0].Legs {
		totalDuration += leg.Duration
	}

	detourTime := totalDuration - directDur
	if detourTime < 0 {
		detourTime = 0 // Should not happen, but safety check
	}

	return detourTime, nil
}

// GetRouteWaypoints returns key points along the route (Origin, Midpoint, Destination).
// This is used to sample search locations along the path.
func (s *RouteService) GetRouteWaypoints(ctx context.Context, origin, destination string) ([]string, error) {
	r := &maps.DirectionsRequest{
		Origin:      origin,
		Destination: destination,
		Mode:        maps.TravelModeDriving,
		Language:    "zh-TW",
		Region:      "TW",
	}

	routes, _, err := s.client.Directions(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("directions error: %w", err)
	}

	if len(routes) == 0 || len(routes[0].Legs) == 0 {
		return nil, fmt.Errorf("no route found")
	}

	leg := routes[0].Legs[0]
	waypoints := []string{origin}

	// Midpoint Strategy: Take the end location of the middle step
	steps := leg.Steps
	if len(steps) > 0 {
		midStep := steps[len(steps)/2]
		midPoint := fmt.Sprintf("%f,%f", midStep.EndLocation.Lat, midStep.EndLocation.Lng)
		waypoints = append(waypoints, midPoint)
	}

	// Destination (Use the initial string or the geocoded address? Use initial string for simplicity/places search)
	// Actually, using the coordinate is safer for "SearchNearby" if the original string was vague.
	// But let's stick to the input format if possible, or coordinate.
	// SearchNearby handles "Lat,Lng" string? Yes, text search query "flower shop near Lat,Lng" works.
	waypoints = append(waypoints, destination)

	return waypoints, nil
}
