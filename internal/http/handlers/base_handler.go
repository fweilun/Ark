// README: Base handler utilities (JSON helpers, error mapping).
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/modules/order"
)

// RespondError writes the canonical {"error": msg} payload with the given
// HTTP status. Mirrors ark/internal/http.RespondError — kept here to avoid
// an import cycle (internal/http imports this package).
func RespondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}

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
		RespondError(c, http.StatusBadRequest, err.Error())
	case order.ErrNotFound:
		RespondError(c, http.StatusNotFound, err.Error())
	case order.ErrInvalidState, order.ErrActiveOrder, order.ErrConflict:
		RespondError(c, http.StatusConflict, err.Error())
	default:
		RespondError(c, http.StatusInternalServerError, "internal error")
	}
}
