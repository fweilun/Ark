// README: Tiny HTTP helpers shared across every handler.
//
// This package exists purely to host the single canonical RespondError
// helper that every HTTP handler calls, so all error responses share
// one source of truth for the wire shape. It is deliberately
// dependency-free — importing ark/internal/http or any module package
// would re-introduce the import cycle we are trying to eliminate, so
// this package MUST stay at the leaves of the dependency graph.
package httpx

import "github.com/gin-gonic/gin"

// ErrorBody is the canonical error payload shape returned by every
// endpoint. Keeping it exported lets swag / OpenAPI tooling reference a
// real Go type instead of guessing from inline gin.H literals.
type ErrorBody struct {
	Error string `json:"error"`
}

// RespondError writes an HTTP error response with the given status
// code and message using the shared {"error": msg} shape. Callers are
// expected to `return` after this call; this helper does not abort the
// Gin chain so existing handler semantics are preserved.
func RespondError(c *gin.Context, status int, msg string) {
	c.JSON(status, ErrorBody{Error: msg})
}
