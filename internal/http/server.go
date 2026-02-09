// README: API gateway; registers HTTP routes and delegates to module services.
package http

import (
	"net/http"

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
	order    *order.Service
	matching *matching.Service
	location *location.Service
	pricing  *pricing.Service
}

func NewServer(deps ServerDeps) *Server {
	return &Server{
		order:    deps.Order,
		matching: deps.Matching,
		location: deps.Location,
		pricing:  deps.Pricing,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/rides/request", s.HandleRequestRide)
	mux.HandleFunc("/api/rides/cancel", s.HandleCancelRide)
	mux.HandleFunc("/api/passenger/order_status/", s.HandleOrderStatus)

	mux.HandleFunc("/api/driver/set_availability", s.HandleDriverAvailability)
	mux.HandleFunc("/api/rides/accept", s.HandleAcceptOrder)
	mux.HandleFunc("/api/rides/reject", s.HandleRejectOrder)
	mux.HandleFunc("/api/rides/start", s.HandleStartTrip)
	mux.HandleFunc("/api/rides/complete", s.HandleCompleteTrip)

	mux.HandleFunc("/api/location/update", s.HandleLocationUpdate)
	return mux
}
