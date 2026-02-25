// README: Auth middleware (stub for MVP).
package middleware

import (
	"context"

	"github.com/gin-gonic/gin"

	"ark/internal/contextkey"
)

// [TODO] Implement real auth with JWT or similar in the future.
// For MVP, user_id is extracted from the X-User-ID header and stored in the request context.

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			ctx := context.WithValue(c.Request.Context(), contextkey.UserID, userID)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}
