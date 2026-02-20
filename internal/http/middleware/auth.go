// README: Auth middleware – validates Firebase ID tokens and populates caller identity in context.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ark/internal/infra"
)

// Context keys used to pass authenticated caller information to handlers.
const (
	CtxUID    = "auth_uid"
	CtxClaims = "auth_claims"
)

// Auth returns a Gin middleware that verifies the Firebase ID token supplied in
// the "Authorization: Bearer <token>" header.  It stores the verified UID and
// custom claims in the Gin context so downstream handlers can perform
// authorisation checks without re-parsing the token.
func Auth(verifier infra.TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid Authorization header"})
			return
		}
		rawToken := strings.TrimPrefix(header, "Bearer ")
		token, err := verifier.VerifyIDToken(c.Request.Context(), rawToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set(CtxUID, token.UID)
		c.Set(CtxClaims, token.Claims)
		c.Next()
	}
}

// CallerUID retrieves the authenticated UID from the Gin context.
func CallerUID(c *gin.Context) string {
	uid, _ := c.Get(CtxUID)
	s, _ := uid.(string)
	return s
}

// CallerRole retrieves the "role" custom claim from the Gin context.
func CallerRole(c *gin.Context) string {
	claims, _ := c.Get(CtxClaims)
	m, ok := claims.(map[string]interface{})
	if !ok {
		return ""
	}
	role, _ := m["role"].(string)
	return role
}
