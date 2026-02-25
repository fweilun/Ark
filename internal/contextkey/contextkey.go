// README: Shared context keys for propagating auth data through request contexts.
package contextkey

// Key is an unexported type for context keys to prevent collisions with other packages.
type Key string

const (
	// UserID is the context key for the authenticated user's ID (set by Auth middleware).
	UserID Key = "user_id"
)
