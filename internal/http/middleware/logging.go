// README: Logging middleware (stub for MVP).
package middleware

import (
	"log"

	"github.com/gin-gonic/gin"
)

// Currently not used
func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("%s %s", c.Request.Method, c.Request.URL.Path)
		c.Next()
	}
}
