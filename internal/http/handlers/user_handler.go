// README: HTTP handlers for the user module.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ark/internal/modules/user"
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

// CreateUser handles POST /users.
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" || req.Email == "" || req.UserType == "" {
		writeError(c, http.StatusBadRequest, "missing fields")
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

// GetUser handles GET /users/:id — retrieves a user by user_id.
func (h *UserHandler) GetUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid user id")
		return
	}
	u, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, u)
}

// GetMe handles GET /me — returns the current user identified by token.
// [TODO] Requires real auth middleware to set "userID" (int) in gin context.
func (h *UserHandler) GetMe(c *gin.Context) {
	raw, exists := c.Get("userID")
	if !exists {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, ok := raw.(int)
	if !ok || id <= 0 {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	u, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, u)
}

// UpdateName handles PATCH /users/:id — updates only the user's name.
func (h *UserHandler) UpdateName(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid user id")
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
	if err := h.svc.UpdateName(c.Request.Context(), id, req.Name); err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"user_id": id})
}

// DeleteUser handles DELETE /users/:id.
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
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
