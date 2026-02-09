// README: Driver-facing HTTP handlers (MVP).
package http

import "net/http"

func (s *Server) HandleDriverAvailability(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) HandleAcceptOrder(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) HandleRejectOrder(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) HandleStartTrip(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) HandleCompleteTrip(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}
