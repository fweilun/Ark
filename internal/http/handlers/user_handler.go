// README: User handler implements CRUD endpoints.
// The user_id is parsed from the X-User-ID header (set by the Firebase auth middleware).
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/modules/user"
)

type UserHandler struct {
	svc *user.Service
}

func NewUserHandler(svc *user.Service) *UserHandler {
	return &UserHandler{svc: svc}
}

type createUserReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	UserType string `json:"user_type"`
}

// Create handles POST /api/v1/users.
// The user_id is obtained from the X-User-ID header set by the Firebase auth middleware.
func (h *UserHandler) Create(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		writeError(c, http.StatusUnauthorized, "missing user id")
		return
	}
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" || req.Email == "" || req.Phone == "" || req.UserType == "" {
		writeError(c, http.StatusBadRequest, "missing fields")
		return
	}
	if err := h.svc.Create(c.Request.Context(), user.CreateCommand{
		UserID:   userID,
		Name:     req.Name,
		Email:    req.Email,
		Phone:    req.Phone,
		UserType: req.UserType,
	}); err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, map[string]any{"user_id": userID})
}

// Get handles GET /api/v1/users/:id.
func (h *UserHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing user id")
		return
	}
	u, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, u)
}

type updateUserReq struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// Update handles PUT /api/v1/users/:id.
func (h *UserHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing user id")
		return
	}
	var req updateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.svc.Update(c.Request.Context(), user.UpdateCommand{
		UserID: id,
		Name:   req.Name,
		Email:  req.Email,
		Phone:  req.Phone,
	}); err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"user_id": id})
}

// Delete handles DELETE /api/v1/users/:id.
func (h *UserHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing user id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		writeUserError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"user_id": id})
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
