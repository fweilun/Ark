// README: Driver HTTP handlers — driver_id always comes from context, never from the request body.
//
// Endpoints:
//
//	POST  /api/driver/create  — create driver profile (driver_id from context, body: license_number)
//	PATCH /api/driver/status  — update driver status  (driver_id from context, body: status)
//
// Auth: The Auth middleware must set "user_id" in the request context before these handlers run.
// Any request without a valid user_id in context is rejected with 401 Unauthorized.
package driver

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler holds the driver HTTP handlers.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler backed by the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type createReq struct {
	LicenseNumber string `json:"license_number"`
}

// Create handles POST /api/driver/create.
// The driver_id is taken from the request context (set by Auth middleware).
// Body: {"license_number": "..."}
func (h *Handler) Create(c *gin.Context) {
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.LicenseNumber == "" {
		writeError(c, http.StatusBadRequest, "missing license_number")
		return
	}

	d, err := h.svc.Create(c.Request.Context(), req.LicenseNumber)
	if err != nil {
		writeDriverError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, map[string]any{
		"driver_id":      d.ID,
		"license_number": d.LicenseNumber,
		"status":         d.Status,
		"rating":         d.Rating,
		"onboarded_at":   d.OnboardedAt,
	})
}

type updateStatusReq struct {
	Status string `json:"status"`
}

// UpdateStatus handles PATCH /api/driver/status.
// The driver_id is taken from the request context (set by Auth middleware).
// Body: {"status": "available"|"on_trip"|"offline"}
func (h *Handler) UpdateStatus(c *gin.Context) {
	var req updateStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Status == "" {
		writeError(c, http.StatusBadRequest, "missing status")
		return
	}

	if err := h.svc.UpdateStatus(c.Request.Context(), req.Status); err != nil {
		writeDriverError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"status": req.Status})
}

func writeJSON(c *gin.Context, status int, v any) {
	c.JSON(status, v)
}

func writeError(c *gin.Context, status int, msg string) {
	writeJSON(c, status, map[string]any{"error": msg})
}

func writeDriverError(c *gin.Context, err error) {
	switch err {
	case ErrForbidden:
		writeError(c, http.StatusUnauthorized, "authentication required")
	case ErrBadRequest:
		writeError(c, http.StatusBadRequest, err.Error())
	case ErrNotFound:
		writeError(c, http.StatusNotFound, err.Error())
	case ErrConflict:
		writeError(c, http.StatusConflict, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal error")
	}
}
