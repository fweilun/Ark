// README: Rating model for trip ratings.
package rating

// Rating stores mutual ratings for a completed trip.
type Rating struct {
	RatingID     int64
	TripID       string // FK to orders.id
	RiderRating  int    // driver rates the passenger (1-5)
	DriverRating int    // passenger rates the driver (1-5)
	Comments     string
}
