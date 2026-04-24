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
//
// @Summary      Register FCM device
// @Description  Registers (or refreshes) an FCM device token for the authenticated user so push notifications can be delivered. Idempotent — re-registering the same token updates the row.
// @Tags         Notification
// @Accept       json
// @Produce      json
// @Security     FirebaseAuth
// @Param        body  body      ensureDeviceReq       true  "FCM token, platform, optional device_id"
// @Success      200   {object}  ensureDeviceResponse
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Failure      500   {object}  httpx.ErrorBody
// @Router       /api/notifications/register [post]
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
// Currently always returns 501 Not Implemented and is not registered on the router.
func (h *NotificationHandler) SendNotification(c *gin.Context) {
	httpx.RespondError(c, http.StatusNotImplemented, "not implemented")
}
