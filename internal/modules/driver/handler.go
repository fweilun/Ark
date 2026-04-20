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
	"time"

	"github.com/gin-gonic/gin"

	"ark/internal/http/dto"
	"ark/internal/httpx"
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
	LicenseNumber string `json:"license_number" binding:"required"`
}

// Create handles POST /api/driver/create.
// The driver_id is taken from the request context (set by Auth middleware).
// Body: {"license_number": "..."}
func (h *Handler) Create(c *gin.Context) {
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	d, err := h.svc.Create(c.Request.Context(), req.LicenseNumber)
	if err != nil {
		writeDriverError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, dto.DriverResponse{
		DriverID:      string(d.ID),
		LicenseNumber: d.LicenseNumber,
		Status:        d.Status,
		Rating:        d.Rating,
		OnboardedAt:   d.OnboardedAt.UTC().Format(time.RFC3339),
	})
}

type updateStatusReq struct {
	Status string `json:"status" binding:"required,oneof=available on_trip offline"`
}

// updateStatusResponse mirrors the legacy {"status": "<new>"} acknowledgement.
type updateStatusResponse struct {
	Status string `json:"status"`
}

// UpdateStatus handles PATCH /api/driver/status.
// The driver_id is taken from the request context (set by Auth middleware).
// Body: {"status": "available"|"on_trip"|"offline"}
func (h *Handler) UpdateStatus(c *gin.Context) {
	var req updateStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.UpdateStatus(c.Request.Context(), req.Status); err != nil {
		writeDriverError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, updateStatusResponse{Status: req.Status})
}

func writeJSON(c *gin.Context, status int, v any) {
	c.JSON(status, v)
}

func writeDriverError(c *gin.Context, err error) {
	switch err {
	case ErrForbidden:
		httpx.RespondError(c, http.StatusUnauthorized, "authentication required")
	case ErrBadRequest:
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
	case ErrNotFound:
		httpx.RespondError(c, http.StatusNotFound, err.Error())
	case ErrConflict:
		httpx.RespondError(c, http.StatusConflict, err.Error())
	default:
		httpx.RespondError(c, http.StatusInternalServerError, "internal error")
	}
}
