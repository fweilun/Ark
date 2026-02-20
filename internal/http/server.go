// README: HTTP server wiring (Gin engine factory).
package http

import (
	"net/http"

	"ark/internal/infra"
	"ark/internal/modules/location"
	"ark/internal/modules/matching"
	"ark/internal/modules/order"
	"ark/internal/modules/pricing"
)

type ServerDeps struct {
	Order    *order.Service
	Matching *matching.Service
	Location *location.Service
	Pricing  *pricing.Service
	Verifier infra.TokenVerifier
}

type Server struct {
	Engine http.Handler
}

func NewServer(deps ServerDeps) *Server {
	engine := NewRouter(deps.Order, deps.Matching, deps.Location, deps.Pricing, deps.Verifier)
	return &Server{Engine: engine}
}

func (s *Server) Routes() http.Handler {
	return s.Engine
}
