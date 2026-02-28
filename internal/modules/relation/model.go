// README: Relation domain model — friendships, requests, and friends.
package relation

import (
	"errors"
	"time"

	"ark/internal/types"
)

// UserID is an alias for the common types.ID used in the relation domain.
type UserID = types.ID

// FriendshipStatus represents the state of a friendship record.
type FriendshipStatus int8

const (
	StatusPending   FriendshipStatus = 0
	StatusAccepted  FriendshipStatus = 1
	StatusRejected  FriendshipStatus = 2
	StatusCancelled FriendshipStatus = 3
)

// Friendship represents a row in the friendships table.
type Friendship struct {
	ID        int64
	UserID    UserID
	FriendID  UserID
	Status    FriendshipStatus
	GroupID   *int
	Remark    *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// FriendRequest represents a pending or processed friend request.
type FriendRequest struct {
	FromUserID UserID
	ToUserID   UserID
	Status     FriendshipStatus
	CreatedAt  time.Time
}

// Friend represents an accepted friendship with the friend's user details.
type Friend struct {
	UserID  UserID
	Name    string
	Phone   string
	Remark  *string
	GroupID *int
	Since   time.Time
}

// User holds the minimal user info returned by search results.
type User struct {
	UserID UserID
	Name   string
	Phone  string
}

var (
	ErrNotFound   = errors.New("relation: not found")
	ErrBadRequest = errors.New("relation: bad request")
	ErrConflict   = errors.New("relation: request already exists")
	ErrForbidden  = errors.New("relation: forbidden")
)
