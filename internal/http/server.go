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
    mux.HandleFunc("/api/passenger/request_ride", s.HandleRequestRide)
    mux.HandleFunc("/api/passenger/cancel_ride", s.HandleCancelRide)
    mux.HandleFunc("/api/passenger/order_status/", s.HandleOrderStatus)

    mux.HandleFunc("/api/driver/set_availability", s.HandleDriverAvailability)
    mux.HandleFunc("/api/driver/accept_order", s.HandleAcceptOrder)
    mux.HandleFunc("/api/driver/reject_order", s.HandleRejectOrder)
    mux.HandleFunc("/api/driver/start_trip", s.HandleStartTrip)
    mux.HandleFunc("/api/driver/complete_trip", s.HandleCompleteTrip)

    mux.HandleFunc("/api/location/update", s.HandleLocationUpdate)
    return mux
}
