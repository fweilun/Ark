// README: Recovery middleware (stub for MVP).
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Currently not used
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recover() != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
