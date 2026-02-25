// README: HTTP server wiring (Gin engine factory).
package http

import (
	"net/http"

	"ark/internal/modules/aiusage"
	"ark/internal/modules/calendar"
	"ark/internal/modules/location"
	"ark/internal/modules/matching"
	"ark/internal/modules/notification"
	"ark/internal/modules/order"
	"ark/internal/modules/pricing"
	"ark/internal/modules/vehicle"
)

type ServerDeps struct {
	Order        *order.Service
	Matching     *matching.Service
	Location     *location.Service
	Pricing      *pricing.Service
	AI           *aiusage.Service
	Notification *notification.Service
	Calendar     *calendar.Service
	Vehicle      *vehicle.Service
}

type Server struct {
	Engine http.Handler
}

func NewServer(deps ServerDeps) *Server {
	engine := NewRouter(deps.Order, deps.Matching, deps.Location, deps.Pricing, deps.AI, deps.Notification, deps.Calendar, deps.Vehicle)
	return &Server{Engine: engine}
}

func (s *Server) Routes() http.Handler {
	return s.Engine
}
