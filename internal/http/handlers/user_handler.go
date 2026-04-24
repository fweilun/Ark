// README: HTTP handlers for the user module.
package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ark/internal/http/dto"
	"ark/internal/http/middleware"
	"ark/internal/httpx"
	"ark/internal/modules/user"
	"ark/internal/types"
)

// UserHandler exposes user CRUD endpoints.
type UserHandler struct {
	svc *user.Service
}

// NewUserHandler creates a UserHandler backed by the given service.
func NewUserHandler(svc *user.Service) *UserHandler {
	return &UserHandler{svc: svc}
}

// --- request types ---

type createUserReq struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Phone    string `json:"phone"`
	UserType string `json:"user_type" binding:"required"`
}

type updateUserNameReq struct {
	Name string `json:"name" binding:"required"`
}

// CreateUser handles POST /api/users.
//
// @Summary      Create user profile
// @Description  Creates the backend user row for a freshly signed-up Firebase account. The caller MUST send a valid Firebase ID Token — the server uses the verified UID from that token as the user_id, so subsequent calls (e.g. GET /api/me) resolve to the same row.
// @Tags         Auth & User
// @Accept       json
// @Produce      json
// @Security     FirebaseAuth
// @Param        body  body      createUserReq    true  "New user payload"
// @Success      201   {object}  dto.UserResponse
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Failure      500   {object}  httpx.ErrorBody
// @Router       /api/users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok || uid == "" {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	u, err := h.svc.Create(c.Request.Context(), user.CreateCommand{
		UserID:   types.ID(uid),
		Name:     req.Name,
		Email:    req.Email,
		Phone:    req.Phone,
		UserType: user.UserType(req.UserType),
	})
	if err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, userToDTO(u))
}

// GetMe handles GET /api/me — returns the current user identified by token.
//
// @Summary      Get current user
// @Description  Returns the profile of the user identified by the Firebase ID Token in the Authorization header.
// @Tags         Auth & User
// @Produce      json
// @Security     FirebaseAuth
// @Success      200  {object}  dto.UserResponse
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      404  {object}  httpx.ErrorBody
// @Router       /api/me [get]
func (h *UserHandler) GetMe(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok || uid == "" {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	u, err := h.svc.GetByID(c.Request.Context(), types.ID(uid))
	if err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, userToDTO(u))
}

// UpdateMe handles PATCH /api/me — updates only the current user's name.
//
// @Summary      Update current user name
// @Description  Patches the display name of the currently authenticated user. Only the name field is editable today.
// @Tags         Auth & User
// @Accept       json
// @Security     FirebaseAuth
// @Param        body  body      updateUserNameReq  true  "Updated name"
// @Success      204
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Failure      404   {object}  httpx.ErrorBody
// @Router       /api/me [patch]
func (h *UserHandler) UpdateMe(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok || uid == "" {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req updateUserNameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.UpdateName(c.Request.Context(), types.ID(uid), req.Name); err != nil {
		writeUserError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DeleteMe handles DELETE /api/me — deletes the current authenticated user.
//
// @Summary      Delete current user
// @Description  Hard-deletes the authenticated user's profile. There is no soft-delete today, so the row is removed immediately.
// @Tags         Auth & User
// @Security     FirebaseAuth
// @Success      204
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      404  {object}  httpx.ErrorBody
// @Router       /api/me [delete]
func (h *UserHandler) DeleteMe(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok || uid == "" {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), types.ID(uid)); err != nil {
		writeUserError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// userToDTO maps an internal *user.User to the public UserResponse shape.
func userToDTO(u *user.User) dto.UserResponse {
	if u == nil {
		return dto.UserResponse{}
	}
	return dto.UserResponse{
		UserID:    string(u.UserID),
		Name:      u.Name,
		Email:     u.Email,
		Phone:     u.Phone,
		UserType:  string(u.UserType),
		CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func writeUserError(c *gin.Context, err error) {
	switch err {
	case user.ErrBadRequest:
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
	case user.ErrNotFound:
		httpx.RespondError(c, http.StatusNotFound, err.Error())
	default:
		httpx.RespondError(c, http.StatusInternalServerError, "internal error")
	}
}
