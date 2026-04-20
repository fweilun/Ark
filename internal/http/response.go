// README: Shared HTTP response helpers. Every error JSON returned by the
// server should go through RespondError so clients see a single stable shape:
//
//	{"error": "<message>"}
//
// Sub-packages that cannot import this package without creating an import
// cycle (e.g. internal/http/handlers and module-owned handler files such as
// internal/modules/relation, internal/modules/driver) re-declare a tiny
// wrapper with identical semantics. Keep the shape in lock-step.
package http

import "github.com/gin-gonic/gin"

// ErrorBody is the canonical error payload shape returned by every endpoint.
type ErrorBody struct {
	Error string `json:"error"`
}

// RespondError writes an HTTP error response with the given status code and
// message using the shared {"error": msg} shape. Callers are still expected
// to `return` after this call; this helper does not abort the Gin chain to
// preserve existing handler semantics.
func RespondError(c *gin.Context, status int, msg string) {
	c.JSON(status, ErrorBody{Error: msg})
}
