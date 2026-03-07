// README: HTTP handler for the ride assistant — POST /api/assistant/ride/messages.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/modules/rideassistant"
)

// RideAssistantHandler exposes the ride assistant over HTTP.
type RideAssistantHandler struct {
	svc *rideassistant.Service
}

// NewRideAssistantHandler creates a RideAssistantHandler backed by the given Service.
func NewRideAssistantHandler(svc *rideassistant.Service) *RideAssistantHandler {
	return &RideAssistantHandler{svc: svc}
}

// HandleMessage processes POST /api/assistant/ride/messages.
// The authenticated user_id comes from the Auth middleware context.
func (h *RideAssistantHandler) HandleMessage(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}

	var req rideassistant.MessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Message == "" {
		writeError(c, http.StatusBadRequest, "message is required")
		return
	}

	resp, err := h.svc.HandleMessage(c.Request.Context(), userID.(string), req)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(c, http.StatusOK, resp)
}
