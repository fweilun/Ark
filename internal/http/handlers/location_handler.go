// README: Location handlers (MVP).
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/http/middleware"
	"ark/internal/modules/location"
	"ark/internal/types"
)

type LocationHandler struct {
	location *location.Service
}

func NewLocationHandler(svc *location.Service) *LocationHandler {
	return &LocationHandler{location: svc}
}

func (h *LocationHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing id")
		return
	}
	// Only the authenticated driver may update their own location.
	if middleware.CallerRole(c) != "driver" {
		writeError(c, http.StatusForbidden, "forbidden: driver role required")
		return
	}
	if middleware.CallerUID(c) != id {
		writeError(c, http.StatusForbidden, "forbidden: id does not match authenticated user")
		return
	}
	_ = h.location.Update(c.Request.Context(), location.Update{UserID: types.ID(id), UserType: "driver"})
	writeJSON(c, http.StatusOK, map[string]any{"status": "ok"})
}
