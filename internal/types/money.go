// README: Common money value object used across modules.
package types

// Money is the project-wide monetary value object.
//
// Unit convention: Amount is stored as TWD minor units ("cents"), i.e. 1/100 of
// a New Taiwan Dollar. Example: NT$500.00 is represented as Money{Amount:
// 50000, Currency: "TWD"}. Database columns backing this (e.g. orders.estimated_fee,
// orders.actual_fee, rates.base_fare, rates.per_km) are BIGINTs in the same
// unit; see migrations/0001_init.sql. Callers converting to a display string
// must divide by 100.
type Money struct {
	// Amount is measured in TWD minor units (cents, i.e. 1/100 TWD).
	// Must be non-negative. Use int64 to avoid floating-point drift.
	Amount int64
	// Currency is the ISO-4217 currency code. The project is TWD-only for now,
	// but the field is kept so non-TWD support can be added without a schema change.
	Currency string
}
