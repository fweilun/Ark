// README: Base handler utilities (JSON helpers, error mapping).
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/httpx"
	"ark/internal/modules/order"
)

// isValidID ensures IDs contain only alphanumeric characters, hyphens, and
// underscores (compatible with both internal hex IDs and Firebase UIDs).
func isValidID(v string) bool {
	if len(v) == 0 || len(v) > 128 {
		return false
	}
	for _, c := range v {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func writeJSON(c *gin.Context, status int, v any) {
	c.JSON(status, v)
}

func writeOrderError(c *gin.Context, err error) {
	switch err {
	case order.ErrBadRequest:
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
	case order.ErrNotFound:
		httpx.RespondError(c, http.StatusNotFound, err.Error())
	case order.ErrInvalidState, order.ErrActiveOrder, order.ErrConflict:
		httpx.RespondError(c, http.StatusConflict, err.Error())
	default:
		httpx.RespondError(c, http.StatusInternalServerError, "internal error")
	}
}
