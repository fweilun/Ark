// README: Unit tests for the health handler (no real DB/Redis required).
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthCheck_NilInfra(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHealthHandler(nil, nil, []string{"order", "matching", "location", "pricing", "ai", "notification", "calendar"})
	r.GET("/health", h.Check)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when infra is nil, got %d", w.Code)
	}

	var resp healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Status != "degraded" {
		t.Errorf("expected status=degraded, got %q", resp.Status)
	}
	if resp.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}

	for _, name := range []string{"order", "matching", "location", "pricing", "ai", "notification", "calendar"} {
		key := "module:" + name
		s, ok := resp.Checks[key]
		if !ok {
			t.Errorf("missing check %q", key)
			continue
		}
		if s.Status != "ok" {
			t.Errorf("check %q: expected status=ok, got %q", key, s.Status)
		}
	}

	for _, key := range []string{"database", "redis"} {
		s, ok := resp.Checks[key]
		if !ok {
			t.Errorf("missing check %q", key)
			continue
		}
		if s.Status != "unconfigured" {
			t.Errorf("check %q: expected status=unconfigured, got %q", key, s.Status)
		}
	}
}
