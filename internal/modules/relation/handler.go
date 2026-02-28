// README: Relation HTTP handlers — all friend-request and friendship endpoints.
//
// Endpoints:
//
//	POST   /api/relations/requests               — send a friend request
//	POST   /api/relations/requests/by-phone      — send a friend request by phone number
//	GET    /api/relations/search                 — search users by name or phone (?q=)
//	GET    /api/relations/requests/received      — list received pending requests
//	GET    /api/relations/requests/sent          — list sent pending requests
//	DELETE /api/relations/requests/:friend_id    — cancel an outgoing request
//	POST   /api/relations/requests/:friend_id/accept — accept a received request
//	POST   /api/relations/requests/:friend_id/reject — reject a received request
//	GET    /api/relations/friends                — list accepted friends
//	DELETE /api/relations/friends/:friend_id     — remove a friend
//	GET    /api/relations/friends/:friend_id/is  — check if :friend_id is a friend
//
// Auth: all routes require the Auth middleware to set user_id in context.
package relation

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler holds the relation HTTP handlers.
type Handler struct {
	svc *Service
}

// NewHandler returns a Handler backed by the given Service.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// --- request types ---

type sendRequestReq struct {
	ToUserID string `json:"to_user_id"`
}

type sendRequestByPhoneReq struct {
	Telephone string `json:"telephone"`
}

// SendRequest handles POST /api/relations/requests.
// The sender's user_id is taken from the request context.
func (h *Handler) SendRequest(c *gin.Context) {
	from, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req sendRequestReq
	if err := c.ShouldBindJSON(&req); err != nil || req.ToUserID == "" {
		writeError(c, http.StatusBadRequest, "missing to_user_id")
		return
	}
	if err := h.svc.Request(c.Request.Context(), from, UserID(req.ToUserID)); err != nil {
		writeRelationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SendRequestByPhone handles POST /api/relations/requests/by-phone.
func (h *Handler) SendRequestByPhone(c *gin.Context) {
	from, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req sendRequestByPhoneReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Telephone == "" {
		writeError(c, http.StatusBadRequest, "missing telephone")
		return
	}
	if err := h.svc.RequestByTelephone(c.Request.Context(), from, req.Telephone); err != nil {
		writeRelationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SearchUsers handles GET /api/relations/search?q=<query>.
func (h *Handler) SearchUsers(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		writeError(c, http.StatusBadRequest, "missing query parameter q")
		return
	}
	users, err := h.svc.SearchUsers(c.Request.Context(), q)
	if err != nil {
		writeRelationError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, users)
}

// ListReceived handles GET /api/relations/requests/received.
func (h *Handler) ListReceived(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	reqs, err := h.svc.ListRequested(c.Request.Context(), uid)
	if err != nil {
		writeRelationError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, reqs)
}

// ListSent handles GET /api/relations/requests/sent.
func (h *Handler) ListSent(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	reqs, err := h.svc.ListSentRequests(c.Request.Context(), uid)
	if err != nil {
		writeRelationError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, reqs)
}

// CancelRequest handles DELETE /api/relations/requests/:friend_id.
func (h *Handler) CancelRequest(c *gin.Context) {
	from, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	to := UserID(c.Param("friend_id"))
	if to == "" {
		writeError(c, http.StatusBadRequest, "missing friend_id")
		return
	}
	if err := h.svc.CancelRequest(c.Request.Context(), from, to); err != nil {
		writeRelationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AcceptRequest handles POST /api/relations/requests/:friend_id/accept.
func (h *Handler) AcceptRequest(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	friendID := UserID(c.Param("friend_id"))
	if friendID == "" {
		writeError(c, http.StatusBadRequest, "missing friend_id")
		return
	}
	if err := h.svc.AcceptRequest(c.Request.Context(), uid, friendID); err != nil {
		writeRelationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RejectRequest handles POST /api/relations/requests/:friend_id/reject.
func (h *Handler) RejectRequest(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	friendID := UserID(c.Param("friend_id"))
	if friendID == "" {
		writeError(c, http.StatusBadRequest, "missing friend_id")
		return
	}
	if err := h.svc.RejectRequest(c.Request.Context(), uid, friendID); err != nil {
		writeRelationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListFriends handles GET /api/relations/friends.
func (h *Handler) ListFriends(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	friends, err := h.svc.ListFriend(c.Request.Context(), uid)
	if err != nil {
		writeRelationError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, friends)
}

// RemoveFriend handles DELETE /api/relations/friends/:friend_id.
func (h *Handler) RemoveFriend(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	friendID := UserID(c.Param("friend_id"))
	if friendID == "" {
		writeError(c, http.StatusBadRequest, "missing friend_id")
		return
	}
	if err := h.svc.RemoveFriend(c.Request.Context(), uid, friendID); err != nil {
		writeRelationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// IsFriend handles GET /api/relations/friends/:friend_id/is.
// Returns {"is_friend": true/false} indicating whether :friend_id is an accepted friend.
func (h *Handler) IsFriend(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	friendID := UserID(c.Param("friend_id"))
	if friendID == "" {
		writeError(c, http.StatusBadRequest, "missing friend_id")
		return
	}
	result, err := h.svc.IsFriend(c.Request.Context(), uid, friendID)
	if err != nil {
		writeRelationError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]bool{"is_friend": result})
}

func writeJSON(c *gin.Context, status int, v any) {
	c.JSON(status, v)
}

func writeError(c *gin.Context, status int, msg string) {
	writeJSON(c, status, map[string]any{"error": msg})
}

func writeRelationError(c *gin.Context, err error) {
	switch err {
	case ErrBadRequest:
		writeError(c, http.StatusBadRequest, err.Error())
	case ErrNotFound:
		writeError(c, http.StatusNotFound, err.Error())
	case ErrConflict:
		writeError(c, http.StatusConflict, err.Error())
	case ErrForbidden:
		writeError(c, http.StatusForbidden, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal error")
	}
}
