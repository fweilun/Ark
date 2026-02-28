// README: User service — CRUD for natural persons (riders and drivers).
package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"ark/internal/types"
)

var (
	ErrNotFound   = errors.New("user: not found")
	ErrBadRequest = errors.New("user: bad request")
)

// Service orchestrates user creation and management.
type Service struct {
	store *Store
}

// NewService creates a Service backed by the given Store.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// CreateCommand holds the fields required to create a new user.
type CreateCommand struct {
	Name     string
	Email    string
	Phone    string
	UserType UserType
}

// Create persists a new user with created_at set to now.
func (s *Service) Create(ctx context.Context, cmd CreateCommand) (*User, error) {
	if cmd.Name == "" || cmd.Email == "" {
		return nil, ErrBadRequest
	}
	if cmd.UserType != UserTypeRider && cmd.UserType != UserTypeDriver {
		return nil, ErrBadRequest
	}
	u := &User{
		UserID:    newID(),
		Name:      cmd.Name,
		Email:     cmd.Email,
		Phone:     cmd.Phone,
		UserType:  cmd.UserType,
		CreatedAt: time.Now(),
	}
	if err := s.store.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// GetByID retrieves a user by their user_id.
func (s *Service) GetByID(ctx context.Context, id types.ID) (*User, error) {
	if id == "" {
		return nil, ErrBadRequest
	}
	return s.store.GetByID(ctx, id)
}

// UpdateName updates only the name of the user with the given id.
func (s *Service) UpdateName(ctx context.Context, id types.ID, name string) error {
	if id == "" || name == "" {
		return ErrBadRequest
	}
	return s.store.UpdateName(ctx, id, name)
}

// Delete removes the user with the given id.
func (s *Service) Delete(ctx context.Context, id types.ID) error {
	if id == "" {
		return ErrBadRequest
	}
	return s.store.Delete(ctx, id)
}

func newID() types.ID {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return types.ID(hex.EncodeToString(b[:]))
}
