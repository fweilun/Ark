// README: User model and type definitions.
package user

import "time"

// User represents a natural person in the system (rider or driver).
type User struct {
	UserID    string
	Name      string
	Email     string
	Phone     string
	UserType  string // 'rider' or 'driver'
	CreatedAt time.Time
}
