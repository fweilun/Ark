// README: Geocoder adapter wrapping maps.RouteService for address → coordinates conversion.
package rideassistant

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"ark/internal/maps"
)

// MapsGeocoder implements Geocoder using the Google Maps RouteService.
type MapsGeocoder struct {
	routeSvc *maps.RouteService
}

// NewMapsGeocoder creates a geocoder backed by the Google Maps API.
func NewMapsGeocoder(routeSvc *maps.RouteService) *MapsGeocoder {
	return &MapsGeocoder{routeSvc: routeSvc}
}

// Geocode converts a text address to lat/lng coordinates.
func (g *MapsGeocoder) Geocode(ctx context.Context, address string) (lat, lng float64, err error) {
	// RouteService.Geocode returns "lat,lng" as a string.
	result, err := g.routeSvc.Geocode(ctx, address)
	if err != nil {
		return 0, 0, err
	}
	parts := strings.SplitN(result, ",", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected geocode result format: %s", result)
	}
	lat, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse lat: %w", err)
	}
	lng, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse lng: %w", err)
	}
	return lat, lng, nil
}
