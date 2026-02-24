// README: Notification service unit tests using an in-memory mock store.
package notification

import (
	"context"
	"testing"
	"time"

	"ark/internal/types"
)

// mockStore is a minimal in-memory NotificationStore for unit tests.
type mockStore struct {
	tokens map[string][]string // userID -> []fcmToken
}

func newMockStore() *mockStore {
	return &mockStore{tokens: make(map[string][]string)}
}

func (m *mockStore) UpsertDevice(_ context.Context, userID types.ID, token, _, _ string) error {
	uid := string(userID)
	for _, t := range m.tokens[uid] {
		if t == token {
			return nil // already present; simulate ON CONFLICT DO UPDATE
		}
	}
	m.tokens[uid] = append(m.tokens[uid], token)
	return nil
}

func (m *mockStore) GetTokensByUserID(_ context.Context, userID types.ID) ([]string, error) {
	return m.tokens[string(userID)], nil
}

func (m *mockStore) DeleteTokens(_ context.Context, tokens []string) error {
	return nil
}

func (m *mockStore) DeleteOutdatedDevices(_ context.Context, _ time.Time) error {
	return nil
}

func TestEnsureDevice(t *testing.T) {
	store := newMockStore()
	svc, err := NewService(store, nil) // no Firebase credentials
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx := context.Background()
	userID := types.ID("usr_svc1")

	if err := svc.EnsureDevice(ctx, userID, "tok1", "android", "dev1"); err != nil {
		t.Fatalf("EnsureDevice: %v", err)
	}

	tokens, _ := store.GetTokensByUserID(ctx, userID)
	if len(tokens) != 1 || tokens[0] != "tok1" {
		t.Fatalf("expected [tok1], got %v", tokens)
	}

	// Second call with same token must not duplicate.
	if err := svc.EnsureDevice(ctx, userID, "tok1", "ios", "dev1"); err != nil {
		t.Fatalf("EnsureDevice (duplicate): %v", err)
	}
	tokens, _ = store.GetTokensByUserID(ctx, userID)
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token after duplicate EnsureDevice, got %d", len(tokens))
	}
}

func TestNotifyUserNoTokens(t *testing.T) {
	store := newMockStore()
	svc, _ := NewService(store, nil)

	// NotifyUser for a user with no registered tokens must succeed without error.
	if err := svc.NotifyUser(context.Background(), "usr_nobody", &NotificationMessage{Title: "Hi", Body: "Test"}); err != nil {
		t.Fatalf("NotifyUser with no tokens: %v", err)
	}
}

func TestNotifyUserNoMessagingClient(t *testing.T) {
	store := newMockStore()
	_ = store.UpsertDevice(context.Background(), "usr_has_token", "tok_x", "android", "")
	svc, _ := NewService(store, nil) // messaging is nil (no credentials)

	// Must return nil — sends are skipped when the messaging client is absent.
	if err := svc.NotifyUser(context.Background(), "usr_has_token", &NotificationMessage{Title: "Hi", Body: "Test"}); err != nil {
		t.Fatalf("NotifyUser with nil messaging client: %v", err)
	}
}

func TestServiceDeleteOutdatedDevices(t *testing.T) {
	store := newMockStore()
	svc, _ := NewService(store, nil)

	// Must delegate to store without error.
	if err := svc.DeleteOutdatedDevices(context.Background(), time.Now()); err != nil {
		t.Fatalf("DeleteOutdatedDevices: %v", err)
	}
}
