// README: Tests for Firebase auth middleware and order handler authorization.
package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"ark/internal/http/middleware"
	"ark/internal/infra"
)

// stubVerifier is a test double for infra.TokenVerifier.
type stubVerifier struct {
	token *infra.FirebaseToken
	err   error
}

func (s *stubVerifier) VerifyIDToken(_ context.Context, _ string) (*infra.FirebaseToken, error) {
	return s.token, s.err
}

func newTestRouter(verifier infra.TokenVerifier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Auth(verifier))
	r.GET("/test", func(c *gin.Context) {
		uid := middleware.CallerUID(c)
		role := middleware.CallerRole(c)
		c.JSON(http.StatusOK, gin.H{"uid": uid, "role": role})
	})
	return r
}

func TestAuth_MissingHeader(t *testing.T) {
	r := newTestRouter(&stubVerifier{token: &infra.FirebaseToken{UID: "user1"}})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuth_InvalidBearerPrefix(t *testing.T) {
	r := newTestRouter(&stubVerifier{token: &infra.FirebaseToken{UID: "user1"}})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Token sometoken")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuth_VerifierError(t *testing.T) {
	r := newTestRouter(&stubVerifier{err: errors.New("bad token")})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalidtoken")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuth_ValidToken_UIDAndRolePopulated(t *testing.T) {
	token := &infra.FirebaseToken{
		UID:    "driver123",
		Claims: map[string]interface{}{"role": "driver"},
	}
	r := newTestRouter(&stubVerifier{token: token})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer validtoken")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty body")
	}
	// Verify response contains the uid and role.
	if !strings.Contains(body, "driver123") {
		t.Errorf("expected uid driver123 in body, got %s", body)
	}
	if !strings.Contains(body, "driver") {
		t.Errorf("expected role driver in body, got %s", body)
	}
}

func TestAuth_ValidToken_NoRoleClaim(t *testing.T) {
	token := &infra.FirebaseToken{
		UID:    "passenger456",
		Claims: map[string]interface{}{},
	}
	r := newTestRouter(&stubVerifier{token: token})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer validtoken")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "passenger456") {
		t.Errorf("expected uid passenger456 in body")
	}
}
