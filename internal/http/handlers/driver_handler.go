// README: Driver HTTP handlers — driver_id always comes from context, never from the request body.
//
// Endpoints:
//
//	POST  /api/driver/create  — create driver profile (driver_id from context, body: license_number)
//	PATCH /api/driver/status  — update driver status  (driver_id from context, body: status)
//
// Auth: The Auth middleware must set "user_id" in the request context before these handlers run.
// Any request without a valid user_id in context is rejected with 401 Unauthorized.
package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ark/internal/http/dto"
	"ark/internal/httpx"
	"ark/internal/modules/driver"
)

// DriverHandler holds the driver HTTP handlers.
type DriverHandler struct {
	svc *driver.Service
}

// NewDriverHandler returns a DriverHandler backed by the given Service.
func NewDriverHandler(svc *driver.Service) *DriverHandler {
	return &DriverHandler{svc: svc}
}

type driverCreateReq struct {
	LicenseNumber string `json:"license_number" binding:"required"`
}

// Create handles POST /api/driver/create.
// The driver_id is taken from the request context (set by Auth middleware).
// Body: {"license_number": "..."}
//
// @Summary      Create driver profile
// @Description  Creates the driver profile for the authenticated user (driver_id always comes from the Firebase token, never from the body). Starts the driver in status "available".
// @Tags         Driver
// @Accept       json
// @Produce      json
// @Security     FirebaseAuth
// @Param        body  body      driverCreateReq     true  "License info"
// @Success      201   {object}  dto.DriverResponse
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Failure      409   {object}  httpx.ErrorBody
// @Router       /api/driver/create [post]
func (h *DriverHandler) Create(c *gin.Context) {
	var req driverCreateReq
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

type driverUpdateStatusReq struct {
	Status string `json:"status" binding:"required,oneof=available on_trip offline"`
}

// driverUpdateStatusResponse mirrors the legacy {"status": "<new>"} acknowledgement.
type driverUpdateStatusResponse struct {
	Status string `json:"status"`
}

// UpdateStatus handles PATCH /api/driver/status.
// The driver_id is taken from the request context (set by Auth middleware).
// Body: {"status": "available"|"on_trip"|"offline"}
//
// @Summary      Update driver status
// @Description  Changes the authenticated driver's status to one of available, on_trip, offline. Returns the new status echoed back.
// @Tags         Driver
// @Accept       json
// @Produce      json
// @Security     FirebaseAuth
// @Param        body  body      driverUpdateStatusReq       true  "New status"
// @Success      200   {object}  driverUpdateStatusResponse
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Failure      404   {object}  httpx.ErrorBody
// @Router       /api/driver/status [patch]
func (h *DriverHandler) UpdateStatus(c *gin.Context) {
	var req driverUpdateStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.UpdateStatus(c.Request.Context(), req.Status); err != nil {
		writeDriverError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, driverUpdateStatusResponse{Status: req.Status})
}

func writeDriverError(c *gin.Context, err error) {
	switch err {
	case driver.ErrForbidden:
		httpx.RespondError(c, http.StatusUnauthorized, "authentication required")
	case driver.ErrBadRequest:
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
	case driver.ErrNotFound:
		httpx.RespondError(c, http.StatusNotFound, err.Error())
	case driver.ErrConflict:
		httpx.RespondError(c, http.StatusConflict, err.Error())
	default:
		httpx.RespondError(c, http.StatusInternalServerError, "internal error")
	}
}
