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
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	// For MVP, no body parsing yet
	_ = h.location.Update(c.Request.Context(), location.Update{UserID: types.ID(userID), UserType: "driver"})
	writeJSON(c, http.StatusOK, map[string]any{"status": "ok"})
}
