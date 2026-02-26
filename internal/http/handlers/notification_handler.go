// README: Notification handler — device registration and push notification endpoints.
package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ark/internal/http/middleware"
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
	FCMToken string `json:"fcm_token"`
	Platform string `json:"platform"`
	DeviceID string `json:"device_id,omitempty"`
}

// EnsureDevice handles POST /api/notifications/register.
// The authenticated user_id is taken from the request context (set by auth middleware).
func (h *NotificationHandler) EnsureDevice(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req ensureDeviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}

	req.FCMToken = strings.TrimSpace(req.FCMToken)
	req.Platform = strings.TrimSpace(req.Platform)

	if req.FCMToken == "" || req.Platform == "" {
		writeError(c, http.StatusBadRequest, "missing fcm_token or platform")
		return
	}

	switch req.Platform {
	case "ios", "android", "web":
	default:
		writeError(c, http.StatusBadRequest, "platform must be one of: ios, android, web")
		return
	}

	if err := h.svc.EnsureDevice(c.Request.Context(), types.ID(userID), req.FCMToken, req.Platform, req.DeviceID); err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(c, http.StatusOK, map[string]any{"message": "device registered"})
}

// SendNotification handles POST /api/notifications/send (staff only — TODO).
func (h *NotificationHandler) SendNotification(c *gin.Context) {
	writeError(c, http.StatusNotImplemented, "not implemented")
}
