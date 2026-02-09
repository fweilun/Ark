// README: Pricing rate definition for each ride type.
package pricing

type Rate struct {
    RideType string
    BaseFare int64
    PerKm    int64
    Currency string
}
