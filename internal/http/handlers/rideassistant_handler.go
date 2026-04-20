// README: HTTP handler for the ride assistant — POST /api/assistant/ride/messages.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/http/dto"
	"ark/internal/http/middleware"
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

// rideAssistantReq mirrors rideassistant.MessageRequest but adds binding tags
// so invalid payloads produce a uniform 400 error. session_id is optional
// (omitempty / no-required-binding); the assistant creates a new session
// when it is absent.
type rideAssistantReq struct {
	Message     string `json:"message" binding:"required"`
	SessionID   string `json:"session_id,omitempty"`
	ContextInfo string `json:"context_info,omitempty"`
}

// HandleMessage processes POST /api/assistant/ride/messages.
// The authenticated user_id comes from the Auth middleware context.
func (h *RideAssistantHandler) HandleMessage(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok || userID == "" {
		RespondError(c, http.StatusUnauthorized, "authentication required")
		return
	}

	var req rideAssistantReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.svc.HandleMessage(c.Request.Context(), userID, rideassistant.MessageRequest{
		Message:     req.Message,
		SessionID:   req.SessionID,
		ContextInfo: req.ContextInfo,
	})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(c, http.StatusOK, rideAssistantToDTO(resp))
}

// rideAssistantToDTO maps an internal rideassistant.MessageResponse to the
// public wire shape. The `session` and `booking` fields remain nullable
// (pointer types with omitempty) so a typical clarification turn does not
// carry either block.
func rideAssistantToDTO(resp *rideassistant.MessageResponse) dto.RideAssistantResponse {
	if resp == nil {
		return dto.RideAssistantResponse{}
	}
	out := dto.RideAssistantResponse{
		Status: resp.Status,
		Reply:  resp.Reply,
	}
	if resp.Session != nil {
		out.Session = &dto.RideAssistantSession{
			ID:            resp.Session.ID,
			Stage:         resp.Session.Stage,
			KnownFields:   resp.Session.KnownFields,
			MissingFields: resp.Session.MissingFields,
		}
	}
	if resp.Booking != nil {
		out.Booking = &dto.RideAssistantBooking{
			OrderID: resp.Booking.OrderID,
			Status:  resp.Booking.Status,
		}
	}
	return out
}
