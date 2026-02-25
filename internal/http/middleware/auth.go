// README: Auth middleware (stub for MVP).
package middleware

import "github.com/gin-gonic/gin"

// [TODO] Implement real auth with JWT or similar in the future.
// For MVP, driver_id is read from the X-Driver-ID request header and stored in Gin context.

const ContextKeyDriverID = "driver_id"

// Auth sets auth-derived claims in the Gin context.
// MVP stub: reads driver_id from the X-Driver-ID header.
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if driverID := c.GetHeader("X-Driver-ID"); driverID != "" {
			c.Set(ContextKeyDriverID, driverID)
		}
		c.Next()
	}
}

// DriverIDFromContext extracts the driver_id stored in the Gin context by the Auth middleware.
// Returns ("", false) when no driver identity is present.
func DriverIDFromContext(c *gin.Context) (string, bool) {
	v, ok := c.Get(ContextKeyDriverID)
	if !ok {
		return "", false
	}
	id, ok := v.(string)
	return id, ok
}
