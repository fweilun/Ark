package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAIChatEndpointGeminiTokenGuard(t *testing.T) {
	t.Logf("[TEST LOG] starting TestAIChatEndpointGeminiTokenGuard")
	loadDotEnv(t)

	dsn := firstNonEmpty(
		strings.TrimSpace(os.Getenv("ARK_TEST_DSN")),
		strings.TrimSpace(os.Getenv("ARK_DB_DSN")),
		"postgres://postgres:postgres@localhost:5432/ark?sslmode=disable",
		"postgres://ark:ark@localhost:5432/ark_test?sslmode=disable",
	)
	baseURL := strings.TrimRight(envOrDefault("ARK_API_BASE_URL", "http://localhost:8080"), "/")
	client := &http.Client{Timeout: 30 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	db, usedDSN := mustConnectDB(t, ctx, dsn)
	t.Cleanup(func() { db.Close() })
	t.Logf("using postgres dsn: %s", redactedDSN(usedDSN))

	uid := fmt.Sprintf("u%d", time.Now().UnixNano())
	currentMonth := time.Now().UTC().Format("2006-01")

	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS ai_usage (
			uid TEXT PRIMARY KEY,
			tokens_remaining INT NOT NULL DEFAULT 100,
			last_reset_month TEXT NOT NULL DEFAULT to_char(now(), 'YYYY-MM')
		)
	`); err != nil {
		t.Fatalf("ensure ai_usage table: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO ai_usage (uid, tokens_remaining, last_reset_month)
		VALUES ($1, 1, $2)
		ON CONFLICT (uid) DO UPDATE SET
			tokens_remaining = EXCLUDED.tokens_remaining,
			last_reset_month = EXCLUDED.last_reset_month
	`, uid, currentMonth); err != nil {
		t.Fatalf("seed ai_usage: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = db.Exec(cleanupCtx, "DELETE FROM ai_usage WHERE uid = $1", uid)
	})

	waitForAPIReady(t, client, baseURL)

	// First call should succeed.
	status1, body1 := callAIChat(t, client, baseURL, uid, "Say hello in one short sentence.")
	if status1 != http.StatusOK {
		t.Fatalf("first call: expected %d, got %d, body=%s", http.StatusOK, status1, string(body1))
	}
	var okResp struct {
		Reply string `json:"reply"`
	}
	if err := json.Unmarshal(body1, &okResp); err != nil {
		t.Fatalf("first call: unmarshal response: %v, raw=%s", err, string(body1))
	}
	if strings.TrimSpace(okResp.Reply) == "" {
		t.Fatalf("first call: expected non-empty reply, raw=%s", string(body1))
	}
	t.Logf("[TEST LOG] Gemini response: %s", okResp.Reply)

	// Second call should fail due to token exhaustion.
	status2, body2 := callAIChat(t, client, baseURL, uid, "Call twice to verify token guard.")
	if status2 != http.StatusTooManyRequests {
		t.Fatalf("second call: expected %d, got %d, body=%s", http.StatusTooManyRequests, status2, string(body2))
	}
	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body2, &errResp); err == nil {
		if !strings.Contains(strings.ToLower(errResp.Error), "insufficient") {
			t.Fatalf("second call: expected insufficient token error, got %q", errResp.Error)
		}
	}
	t.Logf("[TEST LOG] Should fail due to insufficient tokens: %s", errResp.Error)

	var remaining int
	if err := db.QueryRow(ctx, "SELECT tokens_remaining FROM ai_usage WHERE uid = $1", uid).Scan(&remaining); err != nil {
		t.Fatalf("query remaining token: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected tokens_remaining=0 after 2 calls, got %d", remaining)
	}
}

func callAIChat(t *testing.T, client *http.Client, baseURL, uid, message string) (int, []byte) {
	t.Helper()

	payload, err := json.Marshal(map[string]string{
		"uid":     uid,
		"message": message,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/ai/chat", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("call /api/ai/chat: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	return resp.StatusCode, body
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustConnectDB(t *testing.T, parent context.Context, primaryDSN string) (*pgxpool.Pool, string) {
	t.Helper()

	candidates := uniqueNonEmpty(
		primaryDSN,
		strings.TrimSpace(os.Getenv("ARK_TEST_DSN")),
		strings.TrimSpace(os.Getenv("ARK_DB_DSN")),
		"postgres://postgres:postgres@localhost:5432/ark?sslmode=disable",
		"postgres://ark:ark@localhost:5432/ark_test?sslmode=disable",
	)

	var errs []string
	for _, dsn := range candidates {
		ctx, cancel := context.WithTimeout(parent, 5*time.Second)
		db, err := pgxpool.New(ctx, dsn)
		if err != nil {
			cancel()
			errs = append(errs, fmt.Sprintf("%s -> new pool: %v", redactedDSN(dsn), err))
			continue
		}
		if err := db.Ping(ctx); err != nil {
			cancel()
			db.Close()
			errs = append(errs, fmt.Sprintf("%s -> ping: %v", redactedDSN(dsn), err))
			continue
		}
		cancel()
		return db, dsn
	}

	t.Fatalf(
		"cannot connect to postgres. tried DSNs:\n- %s\nhint: run `docker compose up -d postgres redis ark-backend` and ensure host port 5432 is exposed",
		strings.Join(errs, "\n- "),
	)
	return nil, ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func uniqueNonEmpty(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func redactedDSN(dsn string) string {
	at := strings.LastIndex(dsn, "@")
	scheme := strings.Index(dsn, "://")
	if at == -1 || scheme == -1 || at <= scheme+3 {
		return dsn
	}
	return dsn[:scheme+3] + "***:***" + dsn[at:]
}

func waitForAPIReady(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/health", nil)
		if err == nil {
			resp, err := client.Do(req)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("api not ready: GET %s/health did not return 200 in time", baseURL)
}

func loadDotEnv(t *testing.T) {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		return
	}
	path := ""
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if path == "" {
		return
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		if k == "" {
			continue
		}
		if _, ok := os.LookupEnv(k); ok {
			continue
		}
		_ = os.Setenv(k, v)
	}
}
