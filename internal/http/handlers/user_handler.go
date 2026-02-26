// README: User handlers — CRUD for natural persons.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ark/internal/http/middleware"
	"ark/internal/modules/user"
)

// UserHandler handles HTTP requests for user operations.
type UserHandler struct {
	user *user.Service
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(svc *user.Service) *UserHandler {
	return &UserHandler{user: svc}
}

type createUserReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	UserType string `json:"user_type"`
}

// Create handles POST /users.
func (h *UserHandler) Create(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" || req.Email == "" || req.UserType == "" {
		writeError(c, http.StatusBadRequest, "missing fields")
		return
	}
	if req.UserType != string(user.UserTypeRider) && req.UserType != string(user.UserTypeDriver) {
		writeError(c, http.StatusBadRequest, "user_type must be 'rider' or 'driver'")
		return
	}
	id, err := h.user.Create(c.Request.Context(), user.CreateCommand{
		Name:     req.Name,
		Email:    req.Email,
		Phone:    req.Phone,
		UserType: user.UserType(req.UserType),
	})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(c, http.StatusCreated, map[string]any{"user_id": id})
}

// GetByID handles GET /users/:id.
func (h *UserHandler) GetByID(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}
	u, err := h.user.GetByID(c.Request.Context(), id)
	if err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, userResponse(u))
}

// GetMe handles GET /me — returns the authenticated user's profile.
func (h *UserHandler) GetMe(c *gin.Context) {
	rawID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	u, err := h.user.GetByID(c.Request.Context(), id)
	if err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, userResponse(u))
}

type updateNameReq struct {
	Name string `json:"name"`
}

// UpdateName handles PATCH /users/:id — updates the user's name only.
func (h *UserHandler) UpdateName(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}
	var req updateNameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		writeError(c, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.user.UpdateName(c.Request.Context(), id, req.Name); err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"updated": true})
}

// Delete handles DELETE /users/:id.
func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}
	if err := h.user.Delete(c.Request.Context(), id); err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"deleted": true})
}

// --- helpers ---

func parseUserID(c *gin.Context) (int64, bool) {
	raw := c.Param("id")
	if raw == "" {
		writeError(c, http.StatusBadRequest, "missing user id")
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid user id")
		return 0, false
	}
	return id, true
}

func userResponse(u *user.User) map[string]any {
	return map[string]any{
		"user_id":    u.UserID,
		"name":       u.Name,
		"email":      u.Email,
		"phone":      u.Phone,
		"user_type":  u.UserType,
		"created_at": u.CreatedAt,
	}
}

func writeUserError(c *gin.Context, err error) {
	if err == user.ErrNotFound {
		writeError(c, http.StatusNotFound, err.Error())
		return
	}
	writeError(c, http.StatusInternalServerError, "internal error")
}
