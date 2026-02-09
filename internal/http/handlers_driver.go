// README: Driver-facing HTTP handlers (MVP).
package http

import (
	"encoding/json"
	"net/http"

	"ark/internal/modules/order"
	"ark/internal/types"
)

type driverAvailabilityReq struct {
	DriverID          string   `json:"driver_id"`
	IsAvailable       bool     `json:"is_available"`
	CurrentLat        float64  `json:"current_lat"`
	CurrentLng        float64  `json:"current_lng"`
	AcceptedRideTypes []string `json:"accepted_ride_types"`
}

func (s *Server) HandleDriverAvailability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// Matching not implemented yet; accept request for now.
	var req driverAvailabilityReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

type acceptOrderReq struct {
	DriverID string `json:"driver_id"`
	OrderID  string `json:"order_id"`
}

func (s *Server) HandleAcceptOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req acceptOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.DriverID == "" || req.OrderID == "" {
		writeError(w, http.StatusBadRequest, "missing fields")
		return
	}
	err := s.order.Accept(r.Context(), order.AcceptCommand{
		OrderID:  types.ID(req.OrderID),
		DriverID: types.ID(req.DriverID),
	})
	if err != nil {
		writeOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": order.StatusAccepted})
}

func (s *Server) HandleRejectOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// For MVP, treat as no-op
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

type startTripReq struct {
	OrderID string `json:"order_id"`
}

func (s *Server) HandleStartTrip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req startTripReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.OrderID == "" {
		writeError(w, http.StatusBadRequest, "missing order_id")
		return
	}
	err := s.order.Start(r.Context(), order.StartCommand{OrderID: types.ID(req.OrderID)})
	if err != nil {
		writeOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": order.StatusInProgress})
}

type completeTripReq struct {
	OrderID string `json:"order_id"`
}

func (s *Server) HandleCompleteTrip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req completeTripReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.OrderID == "" {
		writeError(w, http.StatusBadRequest, "missing order_id")
		return
	}
	err := s.order.Complete(r.Context(), order.CompleteCommand{OrderID: types.ID(req.OrderID)})
	if err != nil {
		writeOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": order.StatusCompleted})
}
