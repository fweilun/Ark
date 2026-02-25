// README: HTTP handlers for the vehicle module.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/http/middleware"
	"ark/internal/modules/vehicle"
	"ark/internal/types"
)

// VehicleHandler exposes vehicle CRUD endpoints.
type VehicleHandler struct {
	svc *vehicle.Service
}

// NewVehicleHandler creates a VehicleHandler backed by the given service.
func NewVehicleHandler(svc *vehicle.Service) *VehicleHandler {
	return &VehicleHandler{svc: svc}
}

// --- request/response types ---

type createVehicleReq struct {
	Make             string `json:"make"`
	Model            string `json:"model"`
	LicensePlate     string `json:"license_plate"`
	Capacity         int    `json:"capacity"`
	VehicleType      string `json:"vehicle_type"`
	RegistrationDate string `json:"registration_date"` // YYYY-MM-DD
}

type updateVehicleReq struct {
	Make         string `json:"make"`
	Model        string `json:"model"`
	LicensePlate string `json:"license_plate"`
	Capacity     int    `json:"capacity"`
	VehicleType  string `json:"vehicle_type"`
}

// Create handles POST /api/v1/vehicle.
// driver_id is taken from auth context; it must NOT be supplied by the client.
func (h *VehicleHandler) Create(c *gin.Context) {
	driverID, ok := middleware.DriverIDFromContext(c)
	if !ok || driverID == "" {
		writeError(c, http.StatusUnauthorized, "missing driver identity")
		return
	}
	var req createVehicleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Make == "" || req.Model == "" || req.LicensePlate == "" || req.Capacity <= 0 || req.VehicleType == "" {
		writeError(c, http.StatusBadRequest, "missing or invalid fields")
		return
	}
	v, err := h.svc.CreateVehicle(c.Request.Context(), types.ID(driverID), vehicle.CreateVehicleCommand{
		Make:             req.Make,
		Model:            req.Model,
		LicensePlate:     req.LicensePlate,
		Capacity:         req.Capacity,
		VehicleType:      req.VehicleType,
		RegistrationDate: req.RegistrationDate,
	})
	if err != nil {
		writeVehicleError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, vehicleResponse(v))
}

// GetMe handles GET /api/v1/vehicle/me.
func (h *VehicleHandler) GetMe(c *gin.Context) {
	driverID, ok := middleware.DriverIDFromContext(c)
	if !ok || driverID == "" {
		writeError(c, http.StatusUnauthorized, "missing driver identity")
		return
	}
	v, err := h.svc.GetDriverVehicle(c.Request.Context(), types.ID(driverID))
	if err != nil {
		writeVehicleError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, vehicleResponse(v))
}

// Update handles PATCH /api/v1/vehicle.
func (h *VehicleHandler) Update(c *gin.Context) {
	driverID, ok := middleware.DriverIDFromContext(c)
	if !ok || driverID == "" {
		writeError(c, http.StatusUnauthorized, "missing driver identity")
		return
	}
	var req updateVehicleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Make == "" || req.Model == "" || req.LicensePlate == "" || req.Capacity <= 0 || req.VehicleType == "" {
		writeError(c, http.StatusBadRequest, "missing or invalid fields")
		return
	}
	v, err := h.svc.UpdateVehicleInfo(c.Request.Context(), types.ID(driverID), vehicle.UpdateVehicleCommand{
		Make:         req.Make,
		Model:        req.Model,
		LicensePlate: req.LicensePlate,
		Capacity:     req.Capacity,
		VehicleType:  req.VehicleType,
	})
	if err != nil {
		writeVehicleError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, vehicleResponse(v))
}

// Delete handles DELETE /api/v1/vehicle.
func (h *VehicleHandler) Delete(c *gin.Context) {
	driverID, ok := middleware.DriverIDFromContext(c)
	if !ok || driverID == "" {
		writeError(c, http.StatusUnauthorized, "missing driver identity")
		return
	}
	if err := h.svc.DeleteVehicle(c.Request.Context(), types.ID(driverID)); err != nil {
		writeVehicleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func vehicleResponse(v *vehicle.Vehicle) map[string]any {
	return map[string]any{
		"vehicle_id":        v.ID,
		"driver_id":         v.DriverID,
		"make":              v.Make,
		"model":             v.Model,
		"license_plate":     v.LicensePlate,
		"capacity":          v.Capacity,
		"vehicle_type":      v.Type,
		"registration_date": v.RegistrationDate,
	}
}

func writeVehicleError(c *gin.Context, err error) {
	switch err {
	case vehicle.ErrBadRequest:
		writeError(c, http.StatusBadRequest, err.Error())
	case vehicle.ErrNotFound:
		writeError(c, http.StatusNotFound, err.Error())
	case vehicle.ErrConflict:
		writeError(c, http.StatusConflict, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal error")
	}
}
