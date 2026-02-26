// README: HTTP handlers for the user module.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/http/middleware"
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

// --- request/response types ---

type createUserReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	UserType string `json:"user_type"`
}

type updateUserNameReq struct {
	Name string `json:"name"`
}

// CreateUser handles POST /api/users.
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	u, err := h.svc.Create(c.Request.Context(), user.CreateCommand{
		Name:     req.Name,
		Email:    req.Email,
		Phone:    req.Phone,
		UserType: user.UserType(req.UserType),
	})
	if err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, u)
}

// GetMe handles GET /api/me — returns the current user identified by token.
func (h *UserHandler) GetMe(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok || uid == "" {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	u, err := h.svc.GetByID(c.Request.Context(), types.ID(uid))
	if err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, u)
}

// UpdateMe handles PATCH /api/me — updates only the current user's name.
func (h *UserHandler) UpdateMe(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok || uid == "" {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req updateUserNameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		writeError(c, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.svc.UpdateName(c.Request.Context(), types.ID(uid), req.Name); err != nil {
		writeUserError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DeleteMe handles DELETE /api/me — deletes the current authenticated user.
func (h *UserHandler) DeleteMe(c *gin.Context) {
	uid, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok || uid == "" {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), types.ID(uid)); err != nil {
		writeUserError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func writeUserError(c *gin.Context, err error) {
	switch err {
	case user.ErrBadRequest:
		writeError(c, http.StatusBadRequest, err.Error())
	case user.ErrNotFound:
		writeError(c, http.StatusNotFound, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal error")
	}
}
