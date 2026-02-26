// README: User service — CRUD operations for natural persons.
package user

import (
	"context"
	"time"
)

// Service provides business logic for user management.
type Service struct {
	store *Store
}

// NewService creates a new Service.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// CreateCommand holds the input for creating a new user.
type CreateCommand struct {
	Name     string
	Email    string
	Phone    string
	UserType UserType
}

// Create registers a new user with created_at set to now.
func (s *Service) Create(ctx context.Context, cmd CreateCommand) (int64, error) {
	u := &User{
		Name:      cmd.Name,
		Email:     cmd.Email,
		Phone:     cmd.Phone,
		UserType:  cmd.UserType,
		CreatedAt: time.Now(),
	}
	return s.store.Create(ctx, u)
}

// GetByID returns the user with the given id.
func (s *Service) GetByID(ctx context.Context, id int64) (*User, error) {
	return s.store.GetByID(ctx, id)
}

// UpdateName updates only the name of the given user (PATCH semantics).
func (s *Service) UpdateName(ctx context.Context, id int64, name string) error {
	return s.store.UpdateName(ctx, id, name)
}

// Delete removes the user with the given id.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.store.Delete(ctx, id)
}
