// README: Pricing rate definition for each ride type.
package pricing

// Rate encodes the fare schedule for a ride type.
//
// Unit convention: BaseFare and PerKm are in TWD minor units (cents; 1/100
// TWD) to match the project-wide Money convention — see
// internal/types/money.go. Example: a NT$150 base fare is BaseFare=15000.
type Rate struct {
	RideType string
	// BaseFare is the flat fare applied at trip start, in TWD minor units (cents).
	BaseFare int64
	// PerKm is the distance rate, in TWD minor units (cents) per kilometer.
	PerKm    int64
	Currency string
}
