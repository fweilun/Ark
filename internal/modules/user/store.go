// README: User store backed by PostgreSQL.
package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when the requested user does not exist.
var ErrNotFound = errors.New("user not found")

// Store handles persistence for users.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a new Store.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Create inserts a new user and returns the generated user_id.
func (s *Store) Create(ctx context.Context, u *User) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (name, email, phone, user_type, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING user_id`,
		u.Name, u.Email, u.Phone, string(u.UserType), u.CreatedAt,
	).Scan(&id)
	return id, err
}

// GetByID returns the user with the given user_id, or ErrNotFound.
func (s *Store) GetByID(ctx context.Context, id int64) (*User, error) {
	var u User
	var userType string
	err := s.db.QueryRow(ctx, `
		SELECT user_id, name, email, phone, user_type, created_at
		FROM users
		WHERE user_id = $1`, id,
	).Scan(&u.UserID, &u.Name, &u.Email, &u.Phone, &userType, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.UserType = UserType(userType)
	return &u, nil
}

// UpdateName updates only the name field for the given user_id.
func (s *Store) UpdateName(ctx context.Context, id int64, name string) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE users SET name = $1 WHERE user_id = $2`,
		name, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes the user with the given user_id.
func (s *Store) Delete(ctx context.Context, id int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM users WHERE user_id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
