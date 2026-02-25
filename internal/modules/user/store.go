// README: User store backed by PostgreSQL.
package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store handles persistence for the users table.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a Store backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Create inserts a new user and populates UserID from the database.
func (s *Store) Create(ctx context.Context, u *User) error {
	row := s.db.QueryRow(ctx, `
        INSERT INTO users (name, email, phone, user_type, created_at)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING user_id`,
		u.Name, u.Email, u.Phone, string(u.UserType), u.CreatedAt,
	)
	return row.Scan(&u.UserID)
}

// GetByID retrieves a user by their user_id.
func (s *Store) GetByID(ctx context.Context, id int) (*User, error) {
	row := s.db.QueryRow(ctx, `
        SELECT user_id, name, email, phone, user_type, created_at
        FROM users
        WHERE user_id = $1`, id,
	)
	var u User
	err := row.Scan(&u.UserID, &u.Name, &u.Email, &u.Phone, &u.UserType, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateName sets a new name for the user with the given id.
func (s *Store) UpdateName(ctx context.Context, id int, name string) error {
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

// Delete removes a user by their user_id.
func (s *Store) Delete(ctx context.Context, id int) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM users WHERE user_id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
