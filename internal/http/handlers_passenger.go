// README: Passenger-facing HTTP handlers (MVP).
package http

import "net/http"

func (s *Server) HandleRequestRide(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) HandleCancelRide(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) HandleOrderStatus(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "not implemented", http.StatusNotImplemented)
}
