// README: Firebase ID token authentication middleware.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"
)

// contextUserIDKey is a private type used as context key to avoid collisions.
type contextUserIDKey struct{}

// TokenVerifier abstracts Firebase ID token verification, enabling unit testing
// without a live Firebase project. *auth.Client from the Firebase Admin SDK
// satisfies this interface directly.
type TokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error)
}

// Auth returns a Gin middleware that validates a Firebase ID token provided as a
// Bearer token in the Authorization header. On success, the verified Firebase UID
// is injected into the request context; downstream handlers may retrieve it with
// UserIDFromContext.
//
// If verifier is nil, all requests are allowed through without a user_id (dev mode).
func Auth(verifier TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		if verifier == nil {
			// Dev mode: allow all requests through but inject a default user_id
			ctx := context.WithValue(c.Request.Context(), contextUserIDKey{}, "dev-user-id")
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid Authorization header"})
			return
		}
		idToken := strings.TrimPrefix(authHeader, "Bearer ")
		if idToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid Authorization header"})
			return
		}
		token, err := verifier.VerifyIDToken(c.Request.Context(), idToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		ctx := context.WithValue(c.Request.Context(), contextUserIDKey{}, token.UID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// UserIDFromContext retrieves the authenticated Firebase UID stored in ctx by Auth.
// Returns the UID and true if present; otherwise "", false.
func UserIDFromContext(ctx context.Context) (string, bool) {
	uid, ok := ctx.Value(contextUserIDKey{}).(string)
	return uid, ok
}

// WithUserIDContext returns a copy of ctx with the given uid stored under the same
// key used by the Auth middleware. Intended for use in tests only.
func WithUserIDContext(ctx context.Context, uid string) context.Context {
	return context.WithValue(ctx, contextUserIDKey{}, uid)
}
