// README: Auth middleware (stub for MVP).
package middleware

import "github.com/gin-gonic/gin"

// [TODO] Implement real auth with JWT or similar in the future. For MVP, this is a no-op.

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
