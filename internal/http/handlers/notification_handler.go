// README: Notification handler — device registration and push notification endpoints.
package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ark/internal/modules/notification"
	"ark/internal/types"
)

// NotificationHandler handles FCM device registration requests.
type NotificationHandler struct {
	svc notification.NotificationService
}

// NewNotificationHandler returns a NotificationHandler wired to the given service.
func NewNotificationHandler(svc notification.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

type registerDeviceReq struct {
	UserID   string `json:"user_id"`
	FCMToken string `json:"fcm_token"`
	Platform string `json:"platform"`
	DeviceID string `json:"device_id,omitempty"`
}

// RegisterDevice handles POST /api/notifications/register.
func (h *NotificationHandler) RegisterDevice(c *gin.Context) {
	var req registerDeviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}

	req.UserID = strings.TrimSpace(req.UserID)
	req.FCMToken = strings.TrimSpace(req.FCMToken)
	req.Platform = strings.TrimSpace(req.Platform)

	if req.UserID == "" || req.FCMToken == "" || req.Platform == "" {
		writeError(c, http.StatusBadRequest, "missing user_id, fcm_token, or platform")
		return
	}
	if !isValidID(req.UserID) {
		writeError(c, http.StatusBadRequest, "invalid user_id")
		return
	}

	switch req.Platform {
	case "ios", "android", "web":
	default:
		writeError(c, http.StatusBadRequest, "platform must be one of: ios, android, web")
		return
	}

	if err := h.svc.RegisterDevice(c.Request.Context(), types.ID(req.UserID), req.FCMToken, req.Platform, req.DeviceID); err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(c, http.StatusOK, map[string]any{"message": "device registered"})
}

// SendNotification handles POST /api/notifications/send (staff only — TODO).
func (h *NotificationHandler) SendNotification(c *gin.Context) {
	writeError(c, http.StatusNotImplemented, "not implemented")
}
