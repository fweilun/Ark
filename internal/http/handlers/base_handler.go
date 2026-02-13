// README: Base handler utilities (JSON helpers, error mapping).
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/modules/order"
)

type errorResponse struct {
	Error string `json:"error"`
}

// isValidID ensures IDs are hex and 32 chars (matches current ID generator).
func isValidID(v string) bool {
	if len(v) > 32 {
		return false
	}
	for _, c := range v {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			continue
		}
		return false
	}
	return true
}

func writeJSON(c *gin.Context, status int, v any) {
	c.JSON(status, v)
}

func writeError(c *gin.Context, status int, msg string) {
	writeJSON(c, status, errorResponse{Error: msg})
}

func writeOrderError(c *gin.Context, err error) {
	switch err {
	case order.ErrBadRequest:
		writeError(c, http.StatusBadRequest, err.Error())
	case order.ErrNotFound:
		writeError(c, http.StatusNotFound, err.Error())
	case order.ErrInvalidState, order.ErrActiveOrder, order.ErrConflict:
		writeError(c, http.StatusConflict, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal error")
	}
}
