// README: User aggregate — natural person with basic info and role.
package user

import "time"

// UserType represents the role of a user in the system.
type UserType string

const (
	UserTypeRider  UserType = "rider"
	UserTypeDriver UserType = "driver"
)

// User represents a natural person registered in the system.
type User struct {
	UserID    int64
	Name      string
	Email     string
	Phone     string
	UserType  UserType
	CreatedAt time.Time
}
