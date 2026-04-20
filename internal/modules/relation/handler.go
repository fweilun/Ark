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

	"ark/internal/httpx"
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
	ToUserID string `json:"to_user_id" binding:"required"`
}

type sendRequestByPhoneReq struct {
	Telephone string `json:"telephone" binding:"required"`
}

// SendRequest handles POST /api/relations/requests.
// The sender's user_id is taken from the request context.
//
// @Summary      Send friend request
// @Description  Sends a friend request from the authenticated user to the user identified by `to_user_id`.
// @Tags         Relation
// @Accept       json
// @Security     FirebaseAuth
// @Param        body  body      sendRequestReq      true  "Target user id"
// @Success      204
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Failure      404   {object}  httpx.ErrorBody
// @Failure      409   {object}  httpx.ErrorBody
// @Router       /api/relations/requests [post]
func (h *Handler) SendRequest(c *gin.Context) {
	from, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req sendRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.Request(c.Request.Context(), from, UserID(req.ToUserID)); err != nil {
		writeRelationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SendRequestByPhone handles POST /api/relations/requests/by-phone.
//
// @Summary      Send friend request by phone
// @Description  Looks up a user by their registered phone number and sends a friend request from the authenticated user.
// @Tags         Relation
// @Accept       json
// @Security     FirebaseAuth
// @Param        body  body      sendRequestByPhoneReq  true  "Target telephone"
// @Success      204
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Failure      404   {object}  httpx.ErrorBody
// @Failure      409   {object}  httpx.ErrorBody
// @Router       /api/relations/requests/by-phone [post]
func (h *Handler) SendRequestByPhone(c *gin.Context) {
	from, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req sendRequestByPhoneReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.RequestByTelephone(c.Request.Context(), from, req.Telephone); err != nil {
		writeRelationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SearchUsers handles GET /api/relations/search?q=<query>.
//
// @Summary      Search users
// @Description  Free-text user search by name or phone number. Returns a flat JSON array of matching user summaries.
// @Tags         Relation
// @Produce      json
// @Security     FirebaseAuth
// @Param        q    query     string  true  "Search query (name or phone)"
// @Success      200
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Router       /api/relations/search [get]
func (h *Handler) SearchUsers(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing query parameter q")
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
//
// @Summary      List received friend requests
// @Description  Returns pending friend requests the authenticated user has received.
// @Tags         Relation
// @Produce      json
// @Security     FirebaseAuth
// @Success      200
// @Failure      401  {object}  httpx.ErrorBody
// @Router       /api/relations/requests/received [get]
func (h *Handler) ListReceived(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
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
//
// @Summary      List sent friend requests
// @Description  Returns pending friend requests the authenticated user has sent.
// @Tags         Relation
// @Produce      json
// @Security     FirebaseAuth
// @Success      200
// @Failure      401  {object}  httpx.ErrorBody
// @Router       /api/relations/requests/sent [get]
func (h *Handler) ListSent(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
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
//
// @Summary      Cancel outgoing friend request
// @Description  Revokes a pending friend request the authenticated user previously sent to `friend_id`.
// @Tags         Relation
// @Security     FirebaseAuth
// @Param        friend_id  path  string  true  "Target user id"
// @Success      204
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      404  {object}  httpx.ErrorBody
// @Router       /api/relations/requests/{friend_id} [delete]
func (h *Handler) CancelRequest(c *gin.Context) {
	from, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	to := UserID(c.Param("friend_id"))
	if to == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing friend_id")
		return
	}
	if err := h.svc.CancelRequest(c.Request.Context(), from, to); err != nil {
		writeRelationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AcceptRequest handles POST /api/relations/requests/:friend_id/accept.
//
// @Summary      Accept friend request
// @Description  Accepts a pending friend request from `friend_id`. Both users become mutual friends.
// @Tags         Relation
// @Security     FirebaseAuth
// @Param        friend_id  path  string  true  "Requester user id"
// @Success      204
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      404  {object}  httpx.ErrorBody
// @Router       /api/relations/requests/{friend_id}/accept [post]
func (h *Handler) AcceptRequest(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	friendID := UserID(c.Param("friend_id"))
	if friendID == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing friend_id")
		return
	}
	if err := h.svc.AcceptRequest(c.Request.Context(), uid, friendID); err != nil {
		writeRelationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RejectRequest handles POST /api/relations/requests/:friend_id/reject.
//
// @Summary      Reject friend request
// @Description  Rejects a pending friend request from `friend_id`.
// @Tags         Relation
// @Security     FirebaseAuth
// @Param        friend_id  path  string  true  "Requester user id"
// @Success      204
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      404  {object}  httpx.ErrorBody
// @Router       /api/relations/requests/{friend_id}/reject [post]
func (h *Handler) RejectRequest(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	friendID := UserID(c.Param("friend_id"))
	if friendID == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing friend_id")
		return
	}
	if err := h.svc.RejectRequest(c.Request.Context(), uid, friendID); err != nil {
		writeRelationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListFriends handles GET /api/relations/friends.
//
// @Summary      List accepted friends
// @Description  Returns the authenticated user's accepted friends.
// @Tags         Relation
// @Produce      json
// @Security     FirebaseAuth
// @Success      200
// @Failure      401  {object}  httpx.ErrorBody
// @Router       /api/relations/friends [get]
func (h *Handler) ListFriends(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
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
//
// @Summary      Remove friend
// @Description  Breaks the friendship between the authenticated user and `friend_id` (symmetric).
// @Tags         Relation
// @Security     FirebaseAuth
// @Param        friend_id  path  string  true  "Friend user id"
// @Success      204
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      404  {object}  httpx.ErrorBody
// @Router       /api/relations/friends/{friend_id} [delete]
func (h *Handler) RemoveFriend(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	friendID := UserID(c.Param("friend_id"))
	if friendID == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing friend_id")
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
//
// @Summary      Check friendship
// @Description  Returns whether `friend_id` is an accepted friend of the authenticated user. Response shape: `{"is_friend": true|false}`.
// @Tags         Relation
// @Produce      json
// @Security     FirebaseAuth
// @Param        friend_id  path  string  true  "Candidate user id"
// @Success      200
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Router       /api/relations/friends/{friend_id}/is [get]
func (h *Handler) IsFriend(c *gin.Context) {
	uid, ok := userIDFromCtx(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	friendID := UserID(c.Param("friend_id"))
	if friendID == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing friend_id")
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

func writeRelationError(c *gin.Context, err error) {
	switch err {
	case ErrBadRequest:
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
	case ErrNotFound:
		httpx.RespondError(c, http.StatusNotFound, err.Error())
	case ErrConflict:
		httpx.RespondError(c, http.StatusConflict, err.Error())
	case ErrForbidden:
		httpx.RespondError(c, http.StatusForbidden, err.Error())
	default:
		httpx.RespondError(c, http.StatusInternalServerError, "internal error")
	}
}
