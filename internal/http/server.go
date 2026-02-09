// README: HTTP server wiring (entry for http package).
package http

import (
    "net/http"

    "ark/internal/http/handlers"
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
}

type Server struct {
    Handler http.Handler
}

func NewServer(deps ServerDeps) *Server {
    h := NewRouter(deps.Order, deps.Matching, deps.Location, deps.Pricing)
    return &Server{Handler: h}
}

func (s *Server) Routes() http.Handler {
    return s.Handler
}

// Keep this exported for router package usage
var _ = handlers.NewOrderHandler
