// README: Location service handles high-frequency updates with optional snapshot flushing.
package location

import (
	"context"
	"time"

	"ark/internal/types"
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

type Update struct {
	UserID   types.ID
	UserType string
	Position types.Point
}

// Update caches the user's current position in Redis GEO.
func (s *Service) Update(ctx context.Context, u Update) error {
	return s.store.SetGeo(ctx, u.UserID, u.Position, u.UserType)
}

func (s *Service) FlushSnapshot(ctx context.Context, u Update) error {
	snap := Snapshot{
		UserID:     u.UserID,
		UserType:   u.UserType,
		Position:   u.Position,
		RecordedAt: time.Now(),
	}
	return s.store.AppendSnapshot(ctx, snap)
}
