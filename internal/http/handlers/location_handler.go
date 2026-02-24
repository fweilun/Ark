// README: Location handlers (MVP).
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/modules/location"
	"ark/internal/types"
)

type LocationHandler struct {
	location *location.Service
}

func NewLocationHandler(svc *location.Service) *LocationHandler {
	return &LocationHandler{location: svc}
}

type updateLocationReq struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func (h *LocationHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing id")
		return
	}
	var req updateLocationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	_ = h.location.Update(c.Request.Context(), location.Update{
		UserID:   types.ID(id),
		UserType: "driver",
		Position: types.Point{Lat: req.Lat, Lng: req.Lng},
	})
	writeJSON(c, http.StatusOK, map[string]any{"status": "ok"})
}
