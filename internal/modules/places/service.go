package places

import (
	"context"
	"fmt"
	"strings"

	"googlemaps.github.io/maps"
)

// Service interface for places operations.
type Service interface {
	SearchNearby(ctx context.Context, location string, query string, opts *SearchOptions) ([]Place, error)
	SearchAlongRoute(ctx context.Context, waypoints []string, query string, opts *SearchOptions) ([]Place, error)
	SearchAtDestination(ctx context.Context, destinationStr string, query string) ([]Place, error)
}

type placesService struct {
	client *maps.Client
}

// NewService creates a new places service.
func NewService(apiKey string) (Service, error) {
	client, err := maps.NewClient(maps.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create maps client: %w", err)
	}
	return &placesService{client: client}, nil
}

// SearchNearby searches for places matching the query near the given location.
func (s *placesService) SearchNearby(ctx context.Context, location string, query string, opts *SearchOptions) ([]Place, error) {
	fullQuery := query

	if opts != nil && opts.SearchKeywords != "" {
		fullQuery = opts.SearchKeywords + " " + fullQuery
	}

	if location != "" && location != "Current Location" {
		fullQuery = fmt.Sprintf("%s near %s", fullQuery, location)
	}

	isFloristSearch := strings.Contains(strings.ToLower(query), "flower") ||
		strings.Contains(strings.ToLower(query), "florist") ||
		strings.Contains(query, "花")

	r := &maps.TextSearchRequest{
		Query:    fullQuery,
		OpenNow:  true,
		Language: "zh-TW",
		Region:   "TW",
	}

	if isFloristSearch {
		r.Type = "florist"
	}

	resp, err := s.client.TextSearch(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("places api error: %w", err)
	}

	excludedKeywords := []string{"Park", "公園", "Douhua", "豆花", "Shaved Ice", "剉冰", "Soup", "羹", "Tofu", "豆腐"}

	if isFloristSearch {
		excludedKeywords = append(excludedKeywords,
			"Supermarket", "Mart", "Carrefour", "PX Mart",
			"Convenience Store", "Seven", "Family", "Hi-Life", "Ok Mart",
			"全聯", "家樂福", "超市", "便利商店", "超商", "萊爾富", "全家", "統一")
	}

	if opts != nil && len(opts.ExcludeKeywords) > 0 {
		excludedKeywords = append(excludedKeywords, opts.ExcludeKeywords...)
	}

	var results []Place

	for _, result := range resp.Results {
		if result.Rating < 4.0 {
			continue
		}

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

		if len(results) >= 3 {
			break
		}
	}

	return results, nil
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// SearchAlongRoute searches for places near multiple waypoints along a route.
func (s *placesService) SearchAlongRoute(ctx context.Context, waypoints []string, query string, opts *SearchOptions) ([]Place, error) {
	uniquePlaces := make(map[string]Place)
	var allPlaces []Place

	for _, point := range waypoints {
		results, err := s.SearchNearby(ctx, point, query, opts)
		if err != nil {
			continue
		}

		for _, p := range results {
			if _, exists := uniquePlaces[p.PlaceID]; !exists {
				uniquePlaces[p.PlaceID] = p
				allPlaces = append(allPlaces, p)
			}
		}
	}

	if len(allPlaces) == 0 {
		return nil, nil
	}

	return allPlaces, nil
}

// SearchAtDestination searches for reservable restaurants near a destination string.
func (s *placesService) SearchAtDestination(ctx context.Context, destinationStr string, query string) ([]Place, error) {
	fullQuery := query
	if destinationStr != "" {
		fullQuery = fmt.Sprintf("%s near %s", query, destinationStr)
	}

	r := &maps.TextSearchRequest{
		Query:    fullQuery,
		Language: "zh-TW",
		Region:   "TW",
		Type:     "restaurant",
	}

	resp, err := s.client.TextSearch(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("places api (destination) error: %w", err)
	}

	// Exclude hawker stalls and informal food that rarely accept reservations.
	excludedKeywords := []string{
		"小吃", "夜市", "攤", "路邊攤", "自助餐", "快餐", "便當",
		"麵攤", "滷味", "蚵仔煎", "臭豆腐",
	}

	var results []Place
	for _, result := range resp.Results {
		if result.Rating < 4.0 {
			continue
		}

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

		if len(results) >= 3 {
			break
		}
	}

	return results, nil
}
