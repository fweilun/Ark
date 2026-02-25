// README: Driver handler implements CRUD and status/rating update endpoints.
// The driver_id is parsed from the X-User-ID header (set by the Firebase auth middleware)
// for mutating endpoints that operate on the authenticated driver.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/modules/driver"
)

type DriverHandler struct {
	svc *driver.Service
}

func NewDriverHandler(svc *driver.Service) *DriverHandler {
	return &DriverHandler{svc: svc}
}

type createDriverReq struct {
	LicenseNumber string `json:"license_number"`
}

// Create handles POST /api/v1/drivers.
// The driver_id (Firebase UID) is obtained from the X-User-ID header set by the Firebase auth middleware.
func (h *DriverHandler) Create(c *gin.Context) {
	driverID := c.GetHeader("X-User-ID")
	if driverID == "" {
		writeError(c, http.StatusUnauthorized, "missing user id")
		return
	}
	var req createDriverReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.LicenseNumber == "" {
		writeError(c, http.StatusBadRequest, "missing license_number")
		return
	}
	if err := h.svc.Create(c.Request.Context(), driver.CreateCommand{
		DriverID:      driverID,
		LicenseNumber: req.LicenseNumber,
	}); err != nil {
		writeDriverError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, map[string]any{"driver_id": driverID})
}

// Get handles GET /api/v1/drivers/:id.
func (h *DriverHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing driver id")
		return
	}
	d, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		writeDriverError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, d)
}

type updateDriverReq struct {
	LicenseNumber string `json:"license_number"`
	VehicleID     *int64 `json:"vehicle_id,omitempty"`
}

// Update handles PUT /api/v1/drivers/:id.
func (h *DriverHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing driver id")
		return
	}
	var req updateDriverReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.svc.Update(c.Request.Context(), driver.UpdateCommand{
		DriverID:      id,
		LicenseNumber: req.LicenseNumber,
		VehicleID:     req.VehicleID,
	}); err != nil {
		writeDriverError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"driver_id": id})
}

// Delete handles DELETE /api/v1/drivers/:id.
func (h *DriverHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing driver id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeDriverError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"driver_id": id})
}

type updateRatingReq struct {
	Rating float64 `json:"rating"`
}

// UpdateRating handles PUT /api/v1/driver/rating.
// The driver_id is obtained from the X-User-ID header set by the Firebase auth middleware.
func (h *DriverHandler) UpdateRating(c *gin.Context) {
	driverID := c.GetHeader("X-User-ID")
	if driverID == "" {
		writeError(c, http.StatusUnauthorized, "missing user id")
		return
	}
	var req updateRatingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.svc.UpdateRating(c.Request.Context(), driverID, req.Rating); err != nil {
		writeDriverError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"driver_id": driverID, "rating": req.Rating})
}

type updateStatusReq struct {
	Status string `json:"status"`
}

// UpdateStatus handles PUT /api/v1/driver/status.
// The driver_id is obtained from the X-User-ID header set by the Firebase auth middleware.
func (h *DriverHandler) UpdateStatus(c *gin.Context) {
	driverID := c.GetHeader("X-User-ID")
	if driverID == "" {
		writeError(c, http.StatusUnauthorized, "missing user id")
		return
	}
	var req updateStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Status == "" {
		writeError(c, http.StatusBadRequest, "missing status")
		return
	}
	if err := h.svc.UpdateStatus(c.Request.Context(), driverID, driver.Status(req.Status)); err != nil {
		writeDriverError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"driver_id": driverID, "status": req.Status})
}

func writeDriverError(c *gin.Context, err error) {
	switch err {
	case driver.ErrBadRequest:
		writeError(c, http.StatusBadRequest, err.Error())
	case driver.ErrNotFound:
		writeError(c, http.StatusNotFound, err.Error())
	case driver.ErrInvalidStatus:
		writeError(c, http.StatusBadRequest, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal error")
	}
}
