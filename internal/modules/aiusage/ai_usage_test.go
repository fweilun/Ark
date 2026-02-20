// README: AI-usage module tests (lazy reset and quota boundary logic).
package aiusage

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestUseTokenCrossMonthReset verifies that a user with 0 tokens left from a previous month
// is automatically reset and the request succeeds (leaving 99 tokens).
func TestUseTokenCrossMonthReset(t *testing.T) {
	svc, db := setupTestService(t)
	ctx := context.Background()

	// Seed user with 0 tokens from a past month.
	if _, err := db.Exec(ctx, "INSERT INTO ai_usage VALUES ('user_reset', 0, '2000-01')"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := svc.UseToken(ctx, "user_reset"); err != nil {
		t.Fatalf("UseToken after cross-month reset: %v", err)
	}

	var remaining int
	if err := db.QueryRow(ctx, "SELECT tokens_remaining FROM ai_usage WHERE uid = 'user_reset'").Scan(&remaining); err != nil {
		t.Fatalf("query: %v", err)
	}
	if remaining != DefaultTokens-1 {
		t.Fatalf("expected %d tokens remaining, got %d", DefaultTokens-1, remaining)
	}
}

// TestUseTokenInsufficientCheck verifies that a user with 0 tokens in the current month is blocked.
func TestUseTokenInsufficientCheck(t *testing.T) {
	svc, db := setupTestService(t)
	ctx := context.Background()

	// Seed user with 0 tokens for the current month.
	if _, err := db.Exec(ctx, "INSERT INTO ai_usage (uid, tokens_remaining, last_reset_month) VALUES ('user_zero', 0, TO_CHAR(NOW(), 'YYYY-MM'))"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := svc.UseToken(ctx, "user_zero")
	if err != ErrInsufficientTokens {
		t.Fatalf("expected ErrInsufficientTokens, got %v", err)
	}
}

// TestUseTokenNewUser verifies that a user absent from the table is initialised on first call.
func TestUseTokenNewUser(t *testing.T) {
	svc, db := setupTestService(t)
	ctx := context.Background()

	if err := svc.UseToken(ctx, "user_new"); err != nil {
		t.Fatalf("UseToken for new user: %v", err)
	}

	var remaining int
	if err := db.QueryRow(ctx, "SELECT tokens_remaining FROM ai_usage WHERE uid = 'user_new'").Scan(&remaining); err != nil {
		t.Fatalf("query: %v", err)
	}
	if remaining != DefaultTokens-1 {
		t.Fatalf("expected %d tokens remaining after first use, got %d", DefaultTokens-1, remaining)
	}
}

// setupTestService creates a real postgres-backed Service for integration tests.
// It skips the test when ARK_TEST_DSN is not set.
func setupTestService(t *testing.T) (*Service, *pgxpool.Pool) {
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

	if err := applyMigrations(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	if _, err := db.Exec(ctx, "TRUNCATE TABLE ai_usage"); err != nil {
		t.Fatalf("truncate ai_usage: %v", err)
	}

	return NewService(NewStore(db)), db
}

func applyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	migrations := []string{
		"0001_init.sql",
		"0002_schedule.sql",
		"0003_ai_usage.sql",
	}
	for _, name := range migrations {
		path := filepath.Join(root, "migrations", name)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		cleaned := stripSQLComments(string(content))
		for _, stmt := range splitSQL(cleaned) {
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
		stmt := strings.TrimSpace(p)
		if stmt == "" {
			continue
		}
		out = append(out, stmt)
	}
	return out
}
