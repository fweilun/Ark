// README: Notification handler — device registration and push notification endpoints.
package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ark/internal/http/middleware"
	"ark/internal/httpx"
	"ark/internal/modules/notification"
	"ark/internal/types"
)

// NotificationHandler handles FCM device registration requests.
type NotificationHandler struct {
	svc *notification.Service
}

// NewNotificationHandler returns a NotificationHandler wired to the given service.
func NewNotificationHandler(svc *notification.Service) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

type ensureDeviceReq struct {
	FCMToken string `json:"fcm_token" binding:"required"`
	Platform string `json:"platform" binding:"required,oneof=ios android web"`
	DeviceID string `json:"device_id,omitempty"`
}

// ensureDeviceResponse is the registered-device confirmation payload.
type ensureDeviceResponse struct {
	Message string `json:"message"`
}

// EnsureDevice handles POST /api/notifications/register.
// The authenticated user_id is taken from the request context (set by auth middleware).
func (h *NotificationHandler) EnsureDevice(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req ensureDeviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	req.FCMToken = strings.TrimSpace(req.FCMToken)
	req.Platform = strings.TrimSpace(req.Platform)

	if req.FCMToken == "" || req.Platform == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing fcm_token or platform")
		return
	}

	if err := h.svc.EnsureDevice(c.Request.Context(), types.ID(userID), req.FCMToken, req.Platform, req.DeviceID); err != nil {
		httpx.RespondError(c, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(c, http.StatusOK, ensureDeviceResponse{Message: "device registered"})
}

// SendNotification handles POST /api/notifications/send (staff only — TODO).
func (h *NotificationHandler) SendNotification(c *gin.Context) {
	httpx.RespondError(c, http.StatusNotImplemented, "not implemented")
}
