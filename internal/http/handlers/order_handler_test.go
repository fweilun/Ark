// README: Integration tests for order handler authorization checks.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ark/internal/http/handlers"
	httpmiddleware "ark/internal/http/middleware"
	"ark/internal/infra"
	"ark/internal/modules/order"
)

// stubTokenVerifier is a test double for infra.TokenVerifier.
type stubTokenVerifier struct {
	token *infra.FirebaseToken
	err   error
}

func (s *stubTokenVerifier) VerifyIDToken(_ context.Context, _ string) (*infra.FirebaseToken, error) {
	return s.token, s.err
}

// buildTestRouter wires a minimal Gin engine with the auth middleware and the order handler.
func buildTestRouter(verifier infra.TokenVerifier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	// order.NewService(nil, nil) is safe here because all auth checks happen before any
	// service method is called.
	svc := order.NewService(nil, nil)
	r := gin.New()
	r.Use(httpmiddleware.Auth(verifier))
	h := handlers.NewOrderHandler(svc)
	r.POST("/api/orders", h.Create)
	r.POST("/api/orders/:id/accept", h.Accept)
	r.POST("/api/orders/:id/claim", h.Claim)
	return r
}

func makeVerifier(uid, role string) *stubTokenVerifier {
	claims := map[string]interface{}{}
	if role != "" {
		claims["role"] = role
	}
	return &stubTokenVerifier{token: &infra.FirebaseToken{UID: uid, Claims: claims}}
}

func doRequest(r *gin.Engine, method, path string, body interface{}, authHeader string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestCreate_Unauthenticated verifies that requests without a valid token are rejected.
func TestCreate_Unauthenticated(t *testing.T) {
	r := buildTestRouter(&stubTokenVerifier{err: errors.New("no token")})
	w := doRequest(r, http.MethodPost, "/api/orders", map[string]any{
		"passenger_id": "abc123",
		"ride_type":    "standard",
	}, "Bearer badtoken")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestCreate_WrongPassengerID verifies that a passenger cannot create an order for another user.
func TestCreate_WrongPassengerID(t *testing.T) {
	r := buildTestRouter(makeVerifier("realUID", ""))
	w := doRequest(r, http.MethodPost, "/api/orders", map[string]any{
		"passenger_id": "otherUID",
		"ride_type":    "standard",
	}, "Bearer sometoken")
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// TestAccept_RequiresDriverRole checks that a user without the driver role cannot accept an order.
func TestAccept_RequiresDriverRole(t *testing.T) {
	r := buildTestRouter(makeVerifier("driverUID", "")) // no role claim
	w := doRequest(r, http.MethodPost, "/api/orders/abc123abc123abc123abc123abc12301/accept?driver_id=driverUID", nil, "Bearer sometoken")
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// TestAccept_WrongDriverID checks that a driver cannot accept on behalf of another driver.
func TestAccept_WrongDriverID(t *testing.T) {
	r := buildTestRouter(makeVerifier("driverA", "driver"))
	w := doRequest(r, http.MethodPost, "/api/orders/abc123abc123abc123abc123abc12301/accept?driver_id=driverB", nil, "Bearer sometoken")
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// TestClaim_RequiresDriverRole verifies that a passenger cannot claim a scheduled order.
func TestClaim_RequiresDriverRole(t *testing.T) {
	r := buildTestRouter(makeVerifier("passengerUID", "")) // passenger has no role
	w := doRequest(r, http.MethodPost, "/api/orders/abc123abc123abc123abc123abc12301/claim",
		map[string]any{"driver_id": "passengerUID"},
		"Bearer sometoken",
	)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}
