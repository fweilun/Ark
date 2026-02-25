// README: User store backed by PostgreSQL.
package user

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, u *User) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO users (user_id, name, email, phone, user_type)
		VALUES ($1, $2, $3, $4, $5)`,
		u.UserID, u.Name, u.Email, u.Phone, u.UserType,
	)
	return err
}

func (s *Store) Get(ctx context.Context, userID string) (*User, error) {
	row := s.db.QueryRow(ctx, `
		SELECT user_id, name, email, phone, user_type, created_at
		FROM users WHERE user_id = $1`, userID,
	)
	var u User
	err := row.Scan(&u.UserID, &u.Name, &u.Email, &u.Phone, &u.UserType, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) Update(ctx context.Context, u *User) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE users SET name = $1, email = $2, phone = $3
		WHERE user_id = $4`,
		u.Name, u.Email, u.Phone, u.UserID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, userID string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM users WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
