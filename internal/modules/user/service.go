// README: User service implements CRUD for users.
package user

import (
	"context"
	"errors"
)

var (
	ErrNotFound   = errors.New("user not found")
	ErrBadRequest = errors.New("bad request")
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

type CreateCommand struct {
	// UserID is the Firebase UID, parsed from the Authorization header by the auth middleware.
	UserID   string
	Name     string
	Email    string
	Phone    string
	UserType string // 'rider' or 'driver'
}

type UpdateCommand struct {
	UserID string
	Name   string
	Email  string
	Phone  string
}

func (s *Service) Create(ctx context.Context, cmd CreateCommand) error {
	if cmd.UserID == "" || cmd.Name == "" || cmd.Email == "" || cmd.Phone == "" || cmd.UserType == "" {
		return ErrBadRequest
	}
	if cmd.UserType != "rider" && cmd.UserType != "driver" {
		return ErrBadRequest
	}
	return s.store.Create(ctx, &User{
		UserID:   cmd.UserID,
		Name:     cmd.Name,
		Email:    cmd.Email,
		Phone:    cmd.Phone,
		UserType: cmd.UserType,
	})
}

func (s *Service) Get(ctx context.Context, userID string) (*User, error) {
	if userID == "" {
		return nil, ErrBadRequest
	}
	return s.store.Get(ctx, userID)
}

// Update updates name, email, and phone; user_type is immutable.
func (s *Service) Update(ctx context.Context, cmd UpdateCommand) error {
	if cmd.UserID == "" {
		return ErrBadRequest
	}
	return s.store.Update(ctx, &User{
		UserID: cmd.UserID,
		Name:   cmd.Name,
		Email:  cmd.Email,
		Phone:  cmd.Phone,
	})
}

func (s *Service) Delete(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrBadRequest
	}
	return s.store.Delete(ctx, userID)
}
