// README: Relation service — business logic for friend requests and friendships.
package relation

import (
	"context"

	"ark/internal/http/middleware"
)

// Service orchestrates friendship request and management operations.
type Service struct {
	store RelationStore
}

// NewService creates a Service backed by the given RelationStore.
func NewService(store RelationStore) *Service {
	return &Service{store: store}
}

// Request sends a friend request from `from` to `to`.
func (s *Service) Request(ctx context.Context, from, to UserID) error {
	if from == "" || to == "" || from == to {
		return ErrBadRequest
	}
	return s.store.CreateRequest(ctx, from, to)
}

// RequestByTelephone sends a friend request to the user identified by the given phone number.
func (s *Service) RequestByTelephone(ctx context.Context, from UserID, telephone string) error {
	if from == "" || telephone == "" {
		return ErrBadRequest
	}
	target, err := s.store.FindByPhone(ctx, telephone)
	if err != nil {
		return err
	}
	if target.UserID == from {
		return ErrBadRequest
	}
	return s.store.CreateRequest(ctx, from, target.UserID)
}

// SearchUsers returns users whose name or phone contains the query string.
func (s *Service) SearchUsers(ctx context.Context, query string) ([]User, error) {
	if query == "" {
		return nil, ErrBadRequest
	}
	return s.store.SearchUsers(ctx, query)
}

// ListRequested returns pending friend requests received by userID.
func (s *Service) ListRequested(ctx context.Context, userID UserID) ([]FriendRequest, error) {
	if userID == "" {
		return nil, ErrBadRequest
	}
	return s.store.ListReceived(ctx, userID)
}

// CancelRequest cancels a pending outgoing request from `from` to `to`.
func (s *Service) CancelRequest(ctx context.Context, from, to UserID) error {
	if from == "" || to == "" {
		return ErrBadRequest
	}
	return s.store.CancelRequest(ctx, from, to)
}

// AcceptRequest accepts a pending friend request where friendID sent to userID.
func (s *Service) AcceptRequest(ctx context.Context, userID, friendID UserID) error {
	if userID == "" || friendID == "" {
		return ErrBadRequest
	}
	return s.store.UpdateStatus(ctx, friendID, userID, StatusAccepted)
}

// RejectRequest rejects a pending friend request where friendID sent to userID.
func (s *Service) RejectRequest(ctx context.Context, userID, friendID UserID) error {
	if userID == "" || friendID == "" {
		return ErrBadRequest
	}
	return s.store.UpdateStatus(ctx, friendID, userID, StatusRejected)
}

// ListFriend returns the accepted friends list for the given user.
func (s *Service) ListFriend(ctx context.Context, userID UserID) ([]Friend, error) {
	if userID == "" {
		return nil, ErrBadRequest
	}
	return s.store.ListFriends(ctx, userID)
}

// ListSentRequests returns pending friend requests sent by the given user.
func (s *Service) ListSentRequests(ctx context.Context, userID UserID) ([]FriendRequest, error) {
	if userID == "" {
		return nil, ErrBadRequest
	}
	return s.store.ListSent(ctx, userID)
}

// RemoveFriend removes an accepted friendship between userID and friendID.
func (s *Service) RemoveFriend(ctx context.Context, userID, friendID UserID) error {
	if userID == "" || friendID == "" {
		return ErrBadRequest
	}
	return s.store.RemoveFriend(ctx, userID, friendID)
}

// IsFriend reports whether uid1 and uid2 have an accepted friendship.
func (s *Service) IsFriend(ctx context.Context, uid1, uid2 UserID) (bool, error) {
	_, err := s.store.GetFriendship(ctx, uid1, uid2)
	if err == ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// userIDFromCtx extracts the authenticated user's ID from the Go request context.
func userIDFromCtx(ctx context.Context) (UserID, bool) {
	id, ok := middleware.UserIDFromContext(ctx)
	if !ok || id == "" {
		return "", false
	}
	return UserID(id), true
}
