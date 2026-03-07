// README: Driver handler tests — permission enforcement and handler logic.
package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"ark/internal/http/middleware"
	"ark/internal/types"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- in-memory mock store ---

type mockStore struct {
	drivers map[string]*Driver
}

func newMockStore() *mockStore {
	return &mockStore{drivers: make(map[string]*Driver)}
}

func (m *mockStore) Create(_ context.Context, d *Driver) error {
	if _, exists := m.drivers[string(d.ID)]; exists {
		return ErrConflict
	}
	cp := *d
	m.drivers[string(d.ID)] = &cp
	return nil
}

func (m *mockStore) Get(_ context.Context, id types.ID) (*Driver, error) {
	d, ok := m.drivers[string(id)]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *d
	return &cp, nil
}

func (m *mockStore) UpdateRating(_ context.Context, id types.ID, newRating float64) error {
	d, ok := m.drivers[string(id)]
	if !ok {
		return ErrNotFound
	}
	d.Rating = newRating
	return nil
}

func (m *mockStore) UpdateStatusWithLock(_ context.Context, id types.ID, newStatus string) error {
	d, ok := m.drivers[string(id)]
	if !ok {
		return ErrNotFound
	}
	d.Status = newStatus
	return nil
}

// --- test helpers ---

func setupRouter(svc *Service) *gin.Engine {
	r := gin.New()
	h := NewHandler(svc)
	r.PUT("/api/driver/create", h.Create)
	r.PUT("/api/driver/status", h.UpdateStatus)
	return r
}

// withUserID injects a user_id into the request context to simulate an authenticated request.
func withUserID(req *http.Request, userID string) *http.Request {
	ctx := middleware.WithUserIDContext(req.Context(), userID)
	return req.WithContext(ctx)
}

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// --- permission tests ---

func TestCreate_Unauthenticated(t *testing.T) {
	store := newMockStore()
	svc := NewService(store)
	r := setupRouter(svc)

	body := jsonBody(map[string]any{"license_number": "ABC-123"})
	req := httptest.NewRequest(http.MethodPut, "/api/driver/create", body)
	req.Header.Set("Content-Type", "application/json")
	// No user_id in context — unauthenticated.

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateStatus_Unauthenticated(t *testing.T) {
	store := newMockStore()
	svc := NewService(store)
	r := setupRouter(svc)

	body := jsonBody(map[string]any{"status": StatusAvailable})
	req := httptest.NewRequest(http.MethodPut, "/api/driver/status", body)
	req.Header.Set("Content-Type", "application/json")
	// No user_id in context — unauthenticated.

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// --- handler logic tests ---

func TestCreate_MissingLicenseNumber(t *testing.T) {
	store := newMockStore()
	svc := NewService(store)
	r := setupRouter(svc)

	body := jsonBody(map[string]any{"license_number": ""})
	req := httptest.NewRequest(http.MethodPut, "/api/driver/create", body)
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, "driver-1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreate_Success(t *testing.T) {
	store := newMockStore()
	svc := NewService(store)
	r := setupRouter(svc)

	body := jsonBody(map[string]any{"license_number": "AB-1234"})
	req := httptest.NewRequest(http.MethodPut, "/api/driver/create", body)
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, "driver-1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["driver_id"] != "driver-1" {
		t.Errorf("expected driver_id=driver-1, got %v", resp["driver_id"])
	}
	if resp["license_number"] != "AB-1234" {
		t.Errorf("expected license_number=AB-1234, got %v", resp["license_number"])
	}
	if resp["status"] != StatusAvailable {
		t.Errorf("expected status=%s, got %v", StatusAvailable, resp["status"])
	}
	// driver_id in store must be set to context user, never from body.
	d, err := store.Get(context.Background(), types.ID("driver-1"))
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if string(d.ID) != "driver-1" {
		t.Errorf("expected stored driver_id=driver-1, got %s", d.ID)
	}
}

func TestCreate_DuplicateDriver(t *testing.T) {
	store := newMockStore()
	// Pre-seed a driver so the second create conflicts.
	store.drivers["driver-2"] = &Driver{
		ID: "driver-2", LicenseNumber: "XX-0000", Rating: 5.0, Status: StatusAvailable, OnboardedAt: time.Now(),
	}
	svc := NewService(store)
	r := setupRouter(svc)

	body := jsonBody(map[string]any{"license_number": "AB-1234"})
	req := httptest.NewRequest(http.MethodPut, "/api/driver/create", body)
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, "driver-2")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateStatus_InvalidStatus(t *testing.T) {
	store := newMockStore()
	store.drivers["driver-3"] = &Driver{
		ID: "driver-3", LicenseNumber: "ZZ-9999", Rating: 5.0, Status: StatusAvailable, OnboardedAt: time.Now(),
	}
	svc := NewService(store)
	r := setupRouter(svc)

	body := jsonBody(map[string]any{"status": "flying"})
	req := httptest.NewRequest(http.MethodPut, "/api/driver/status", body)
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, "driver-3")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateStatus_Success(t *testing.T) {
	store := newMockStore()
	store.drivers["driver-4"] = &Driver{
		ID: "driver-4", LicenseNumber: "CD-5678", Rating: 5.0, Status: StatusAvailable, OnboardedAt: time.Now(),
	}
	svc := NewService(store)
	r := setupRouter(svc)

	body := jsonBody(map[string]any{"status": StatusOnTrip})
	req := httptest.NewRequest(http.MethodPut, "/api/driver/status", body)
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, "driver-4")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != StatusOnTrip {
		t.Errorf("expected status=%s, got %v", StatusOnTrip, resp["status"])
	}

	// Verify store was updated.
	d, err := store.Get(context.Background(), types.ID("driver-4"))
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if d.Status != StatusOnTrip {
		t.Errorf("expected stored status=%s, got %s", StatusOnTrip, d.Status)
	}
}

func TestUpdateStatus_MissingBody(t *testing.T) {
	store := newMockStore()
	svc := NewService(store)
	r := setupRouter(svc)

	body := jsonBody(map[string]any{"status": ""})
	req := httptest.NewRequest(http.MethodPut, "/api/driver/status", body)
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, "driver-5")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDriverIDNeverFromBody verifies that the driver_id cannot be overridden via the request body.
// Even if a malicious client sends a "driver_id" field, the handler must ignore it and use the context value.
func TestDriverIDNeverFromBody(t *testing.T) {
	store := newMockStore()
	svc := NewService(store)
	r := setupRouter(svc)

	// Body includes a "driver_id" field — the handler must ignore it.
	body := jsonBody(map[string]any{"license_number": "EF-1111", "driver_id": "attacker-id"})
	req := httptest.NewRequest(http.MethodPut, "/api/driver/create", body)
	req.Header.Set("Content-Type", "application/json")
	req = withUserID(req, "real-driver-6")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Must use context user_id, not the body's driver_id.
	if _, err := store.Get(context.Background(), types.ID("attacker-id")); err == nil {
		t.Error("store must not contain attacker-id; driver_id must come from context only")
	}
	if _, err := store.Get(context.Background(), types.ID("real-driver-6")); err != nil {
		t.Error("store must contain real-driver-6 (from context)")
	}
}

// --- service-level tests ---

func TestUpdateRating_OutOfRange(t *testing.T) {
	store := newMockStore()
	svc := NewService(store)
	ctx := context.Background()

	if err := svc.UpdateRating(ctx, "any", 6.0); err != ErrBadRequest {
		t.Errorf("expected ErrBadRequest for rating > 5, got %v", err)
	}
	if err := svc.UpdateRating(ctx, "any", -1.0); err != ErrBadRequest {
		t.Errorf("expected ErrBadRequest for rating < 0, got %v", err)
	}
}

func TestDriverInfo_NotFound(t *testing.T) {
	store := newMockStore()
	svc := NewService(store)

	_, err := svc.DriverInfo(context.Background(), "nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
