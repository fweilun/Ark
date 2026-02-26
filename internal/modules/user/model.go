// README: User domain model.
package user

import (
	"time"

	"ark/internal/types"
)

// UserType distinguishes riders from drivers.
type UserType string

const (
	UserTypeRider  UserType = "rider"
	UserTypeDriver UserType = "driver"
)

// User represents a natural person in the system.
type User struct {
	UserID    types.ID
	Name      string
	Email     string
	Phone     string
	UserType  UserType
	CreatedAt time.Time
}
