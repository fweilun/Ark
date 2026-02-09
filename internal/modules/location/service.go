// README: Location service handles high-frequency updates with optional snapshot flushing.
package location

import (
	"context"
	"errors"
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

func (s *Service) Update(ctx context.Context, u Update) error {
	return errors.New("not implemented")
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
