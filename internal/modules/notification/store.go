// README: Notification store backed by PostgreSQL for device token persistence.
package notification

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ark/internal/types"
)

// NotificationStore defines persistence operations for FCM device tokens.
type NotificationStore interface {
	// UpsertDevice adds or updates a device token (using ON CONFLICT on unique key).
	UpsertDevice(ctx context.Context, userID types.ID, token, platform, deviceID string) error

	// GetTokensByUserID returns all active FCM tokens for a user.
	GetTokensByUserID(ctx context.Context, userID types.ID) ([]string, error)

	// UpdateLastSeen updates last_seen_at by device_id (ignored if deviceID is empty).
	UpdateLastSeen(ctx context.Context, userID types.ID, deviceID string) error

	// DeleteTokens removes the given FCM tokens in bulk.
	DeleteTokens(ctx context.Context, tokens []string) error

	// DeleteOutdatedDevices removes devices whose last_seen_at is before the given time.
	DeleteOutdatedDevices(ctx context.Context, before time.Time) error
}

// Store is the PostgreSQL implementation of NotificationStore.
type Store struct {
	db *pgxpool.Pool
}

// NewStore returns a Store backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// UpsertDevice inserts or updates a device token row.
func (s *Store) UpsertDevice(ctx context.Context, userID types.ID, token, platform, deviceID string) error {
	var devID *string
	if deviceID != "" {
		devID = &deviceID
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_fcm_tokens (user_id, fcm_token, platform, device_id, last_seen_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (user_id, fcm_token)
		DO UPDATE SET
			platform     = EXCLUDED.platform,
			device_id    = EXCLUDED.device_id,
			last_seen_at = NOW(),
			updated_at   = NOW()
	`, string(userID), token, platform, devID)
	return err
}

// GetTokensByUserID returns all FCM tokens registered for the given user.
func (s *Store) GetTokensByUserID(ctx context.Context, userID types.ID) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT fcm_token FROM user_fcm_tokens WHERE user_id = $1
	`, string(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// UpdateLastSeen updates the last_seen_at timestamp for the device identified by deviceID.
// If deviceID is empty, no update is performed.
func (s *Store) UpdateLastSeen(ctx context.Context, userID types.ID, deviceID string) error {
	if deviceID == "" {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		UPDATE user_fcm_tokens
		SET last_seen_at = NOW(), updated_at = NOW()
		WHERE user_id = $1 AND device_id = $2
	`, string(userID), deviceID)
	return err
}

// DeleteTokens removes the specified FCM tokens from the database.
func (s *Store) DeleteTokens(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		DELETE FROM user_fcm_tokens WHERE fcm_token = ANY($1)
	`, tokens)
	return err
}

// DeleteOutdatedDevices removes device rows whose last_seen_at is before the given time.
func (s *Store) DeleteOutdatedDevices(ctx context.Context, before time.Time) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM user_fcm_tokens WHERE last_seen_at < $1
	`, before)
	return err
}
