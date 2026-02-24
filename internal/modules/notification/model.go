// README: Notification domain models.
package notification

import (
	"time"

	"ark/internal/types"
)

// UserDevice represents a registered FCM device token for a user.
type UserDevice struct {
	ID         int64     `json:"id" db:"id"`
	UserID     types.ID  `json:"user_id" db:"user_id"`
	Token      string    `json:"token" db:"fcm_token"`
	Platform   string    `json:"platform" db:"platform"`               // ios, android, web
	DeviceID   *string   `json:"device_id,omitempty" db:"device_id"`   // 指針允許 null
	LastSeenAt time.Time `json:"last_seen_at" db:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// UserNotificationProfile is an aggregate view of all active tokens for a user (query only, not stored).
type UserNotificationProfile struct {
	UserID    types.ID `json:"user_id"`
	FCMTokens []string `json:"fcm_tokens"`
}
