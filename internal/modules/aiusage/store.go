package aiusage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store handles ai_usage persistence.
type Store struct {
	db *pgxpool.Pool
}

// NewStore returns a Store backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// UseToken atomically checks the monthly quota and deducts one token.
// It resets the counter to DefaultTokens when last_reset_month is behind the current month.
// Returns ErrInsufficientTokens when 0 rows are updated (quota exhausted or user absent).
func (s *Store) UseToken(ctx context.Context, uid string) error {
	now := time.Now().Format("2006-01")

	tag, err := s.db.Exec(ctx, `
		UPDATE ai_usage SET
			tokens_remaining = CASE WHEN last_reset_month != $1 THEN $2 - 1 ELSE tokens_remaining - 1 END,
			last_reset_month = $1
		WHERE uid = $3 AND (last_reset_month < $1 OR tokens_remaining > 0)
	`, now, DefaultTokens, uid)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInsufficientTokens
	}
	return nil
}

// EnsureUser inserts a new ai_usage row for uid with the default token allowance.
// If the row already exists the insert is silently skipped (ON CONFLICT DO NOTHING).
func (s *Store) EnsureUser(ctx context.Context, uid string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO ai_usage (uid, tokens_remaining, last_reset_month)
		VALUES ($1, $2, $3)
		ON CONFLICT (uid) DO NOTHING
	`, uid, DefaultTokens, time.Now().Format("2006-01"))
	return err
}
