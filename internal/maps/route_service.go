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
// Pass a non-zero departureTime to get a traffic-aware estimate (DurationInTraffic).
// If departureTime is zero, the API uses current conditions.
func (s *RouteService) GetTravelEstimate(ctx context.Context, origin, destination string, departureTime time.Time) (time.Duration, string, error) {
	r := &maps.DirectionsRequest{
		Origin:      origin,
		Destination: destination,
		Mode:        maps.TravelModeDriving,
		Language:    "zh-TW",
		Region:      "TW",
	}
	if !departureTime.IsZero() {
		r.DepartureTime = fmt.Sprintf("%d", departureTime.Unix())
	}

	routes, _, err := s.client.Directions(ctx, r)

	// Fallback: If no route found, try Geocoding the endpoints first.
	if err != nil || len(routes) == 0 {
		geoOrigin, err1 := s.Geocode(ctx, origin)
		geoDest, err2 := s.Geocode(ctx, destination)
		if err1 == nil && err2 == nil {
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
	// Prefer traffic-aware duration when available (requires departure_time).
	duration := leg.Duration
	if leg.DurationInTraffic > 0 {
		duration = leg.DurationInTraffic
	}
	return duration, leg.Distance.HumanReadable, nil
}

// GetDetourEstimate calculates the extra time needed to add a stop.
// departureTime is used for traffic-aware estimation (pass zero for current conditions).
func (s *RouteService) GetDetourEstimate(ctx context.Context, origin, stop, destination string, departureTime time.Time) (time.Duration, error) {
	directDur, _, err := s.GetTravelEstimate(ctx, origin, destination, departureTime)
	if err != nil {
		return 0, fmt.Errorf("failed to get direct route: %w", err)
	}

	r := &maps.DirectionsRequest{
		Origin:      origin,
		Destination: destination,
		Waypoints:   []string{stop},
		Mode:        maps.TravelModeDriving,
		Language:    "zh-TW",
		Region:      "TW",
	}
	if !departureTime.IsZero() {
		r.DepartureTime = fmt.Sprintf("%d", departureTime.Unix())
	}

	routes, _, err := s.client.Directions(ctx, r)
	if err != nil {
		return 0, fmt.Errorf("failed to get detour route: %w", err)
	}
	if len(routes) == 0 || len(routes[0].Legs) == 0 {
		return 0, fmt.Errorf("no detour route found")
	}

	var totalDuration time.Duration
	for _, leg := range routes[0].Legs {
		d := leg.Duration
		if leg.DurationInTraffic > 0 {
			d = leg.DurationInTraffic
		}
		totalDuration += d
	}

	detourTime := totalDuration - directDur
	if detourTime < 0 {
		detourTime = 0
	}
	return detourTime, nil
}

// GetRouteWaypoints returns key points along the route (Origin, Midpoint, Destination).
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
	steps := leg.Steps
	if len(steps) > 0 {
		midStep := steps[len(steps)/2]
		midPoint := fmt.Sprintf("%f,%f", midStep.EndLocation.Lat, midStep.EndLocation.Lng)
		waypoints = append(waypoints, midPoint)
	}
	waypoints = append(waypoints, destination)
	return waypoints, nil
}
