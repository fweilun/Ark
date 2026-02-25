package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"

	"ark/internal/http/middleware"
)

// fakeVerifier is a test double for TokenVerifier.
type fakeVerifier struct {
	uid string
	err error
}

func (f *fakeVerifier) VerifyIDToken(_ context.Context, _ string) (*auth.Token, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &auth.Token{UID: f.uid}, nil
}

func newTestRouter(verifier middleware.TokenVerifier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Auth(verifier))
	r.GET("/test", func(c *gin.Context) {
		uid, ok := middleware.UserIDFromContext(c.Request.Context())
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no user_id in context"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user_id": uid})
	})
	return r
}

func TestAuth_ValidToken(t *testing.T) {
	r := newTestRouter(&fakeVerifier{uid: "firebase-uid-abc123"})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_MissingHeader(t *testing.T) {
	r := newTestRouter(&fakeVerifier{uid: "firebase-uid-abc123"})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_InvalidBearerFormat(t *testing.T) {
	r := newTestRouter(&fakeVerifier{uid: "firebase-uid-abc123"})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Token sometoken") // wrong scheme
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	r := newTestRouter(&fakeVerifier{err: errors.New("token expired")})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer expired-token")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_NilVerifier_PassesThrough(t *testing.T) {
	r := newTestRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Nil verifier bypasses auth; handler sees no user_id and returns 401 from handler logic.
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (no user_id from nil verifier), got %d", w.Code)
	}
}

func TestUserIDFromContext_Empty(t *testing.T) {
	_, ok := middleware.UserIDFromContext(context.Background())
	if ok {
		t.Fatal("expected false for empty context")
	}
}

func TestUserIDFromContext_WithValue(t *testing.T) {
	r := newTestRouter(&fakeVerifier{uid: "user-xyz"})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
