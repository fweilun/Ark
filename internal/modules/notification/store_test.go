// README: Notification store tests backed by real PostgreSQL.
package notification

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ark/internal/types"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestUpsertAndGetTokens(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	userID := types.ID("usr_store1")

	// Insert a new device.
	if err := store.UpsertDevice(ctx, userID, "token_a", "android", "device_1"); err != nil {
		t.Fatalf("UpsertDevice: %v", err)
	}
	tokens, err := store.GetTokensByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetTokensByUserID: %v", err)
	}
	if len(tokens) != 1 || tokens[0] != "token_a" {
		t.Fatalf("expected [token_a], got %v", tokens)
	}

	// Upsert same token — must not duplicate.
	if err := store.UpsertDevice(ctx, userID, "token_a", "ios", "device_1"); err != nil {
		t.Fatalf("UpsertDevice (upsert): %v", err)
	}
	tokens, _ = store.GetTokensByUserID(ctx, userID)
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token after upsert, got %d", len(tokens))
	}

	// Insert a second token.
	if err := store.UpsertDevice(ctx, userID, "token_b", "web", ""); err != nil {
		t.Fatalf("UpsertDevice (second): %v", err)
	}
	tokens, _ = store.GetTokensByUserID(ctx, userID)
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
}

func TestDeleteTokens(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	userID := types.ID("usr_del")

	_ = store.UpsertDevice(ctx, userID, "tok_del1", "android", "")
	_ = store.UpsertDevice(ctx, userID, "tok_del2", "ios", "")

	if err := store.DeleteTokens(ctx, []string{"tok_del1"}); err != nil {
		t.Fatalf("DeleteTokens: %v", err)
	}
	tokens, _ := store.GetTokensByUserID(ctx, userID)
	if len(tokens) != 1 || tokens[0] != "tok_del2" {
		t.Fatalf("expected [tok_del2], got %v", tokens)
	}

	// Empty slice must be a no-op.
	if err := store.DeleteTokens(ctx, nil); err != nil {
		t.Fatalf("DeleteTokens(nil): %v", err)
	}
}

func TestDeleteOutdatedDevices(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()
	userID := types.ID("usr_outdated")

	_ = store.UpsertDevice(ctx, userID, "tok_old", "android", "")

	// A cutoff in the future should remove the row (last_seen_at defaults to NOW()).
	future := time.Now().Add(1 * time.Minute)
	if err := store.DeleteOutdatedDevices(ctx, future); err != nil {
		t.Fatalf("DeleteOutdatedDevices: %v", err)
	}
	tokens, _ := store.GetTokensByUserID(ctx, userID)
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens after deleting outdated, got %d", len(tokens))
	}
}

// setupTestStore connects to Postgres, applies migrations, truncates the token
// table, and returns a Store. The test is skipped when ARK_TEST_DSN is unset.
func setupTestStore(t *testing.T) *Store {
	t.Helper()

	dsn := os.Getenv("ARK_TEST_DSN")
	if dsn == "" {
		t.Skip("ARK_TEST_DSN not set; skipping DB-backed tests")
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := applyNotificationMigration(ctx, db); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	if _, err := db.Exec(ctx, "TRUNCATE TABLE user_fcm_tokens"); err != nil {
		t.Fatalf("truncate user_fcm_tokens: %v", err)
	}

	return NewStore(db)
}

func applyNotificationMigration(ctx context.Context, db *pgxpool.Pool) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	migrations := []string{
		"0001_init.sql",
		"0002_schedule.sql",
		"0003_ai_usage.sql",
		"0004_notifications.sql",
	}
	for _, name := range migrations {
		content, err := os.ReadFile(filepath.Join(root, "migrations", name))
		if err != nil {
			return err
		}
		for _, stmt := range splitSQL(stripSQLComments(string(content))) {
			if _, err := db.Exec(ctx, stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}

func stripSQLComments(input string) string {
	var b strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		b.WriteString(scanner.Text())
		b.WriteString("\n")
	}
	return b.String()
}

func splitSQL(input string) []string {
	parts := strings.Split(input, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if stmt := strings.TrimSpace(p); stmt != "" {
			out = append(out, stmt)
		}
	}
	return out
}
