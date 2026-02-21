// Package location — geo_utils contains pure geographic computation helpers.
package location

import "math"

const earthRadiusKm = 6371.0

// haversineKm returns the great-circle distance in kilometres between two
// points specified in decimal degrees.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	dLat := degreesToRadians(lat2 - lat1)
	dLng := degreesToRadians(lng2 - lng1)

	rLat1 := degreesToRadians(lat1)
	rLat2 := degreesToRadians(lat2)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rLat1)*math.Cos(rLat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180.0
}

// sortByDistance performs an insertion sort (fine for small N) on any slice
// where each element exposes a distance via the accessor function.
func sortByDistance[T any](items []T, dist func(T) float64) {
	for i := 1; i < len(items); i++ {
		key := items[i]
		j := i - 1
		for j >= 0 && dist(items[j]) > dist(key) {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = key
	}
}
