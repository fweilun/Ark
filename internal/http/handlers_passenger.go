// README: Passenger-facing HTTP handlers (MVP).
package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"ark/internal/modules/order"
	"ark/internal/types"
)

type requestRideReq struct {
	PassengerID string  `json:"passenger_id"`
	PickupLat   float64 `json:"pickup_lat"`
	PickupLng   float64 `json:"pickup_lng"`
	DropoffLat  float64 `json:"dropoff_lat"`
	DropoffLng  float64 `json:"dropoff_lng"`
	RideType    string  `json:"ride_type"`
    DurationMin float64 `json:"duration_min"`
    Weather     string  `json:"weather"`  // "rain", "heavy_rain", "normal"
    CarType     string  `json:"car_type"` // "lucky_cat", "normal"
    Tolls       float64 `json:"tolls"`
}

func (s *Server) HandleRequestRide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req requestRideReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.PassengerID == "" || req.RideType == "" {
		writeError(w, http.StatusBadRequest, "missing fields")
		return
	}
	id, err := s.order.Create(r.Context(), order.CreateCommand{
		PassengerID: types.ID(req.PassengerID),
		Pickup:      types.Point{Lat: req.PickupLat, Lng: req.PickupLng},
		Dropoff:     types.Point{Lat: req.DropoffLat, Lng: req.DropoffLng},
		RideType:    req.RideType,
        DurationMin: req.DurationMin,
        Weather:     req.Weather,
        CarType:     req.CarType,
        Tolls:       req.Tolls,
	})
	if err != nil {
		writeOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"order_id": id,
		"status":   order.StatusCreated,
	})
}

type cancelRideReq struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}

func (s *Server) HandleCancelRide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req cancelRideReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.OrderID == "" {
		writeError(w, http.StatusBadRequest, "missing order_id")
		return
	}
	err := s.order.Cancel(r.Context(), order.CancelCommand{
		OrderID:   types.ID(req.OrderID),
		ActorType: "passenger",
		Reason:    req.Reason,
	})
	if err != nil {
		writeOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": order.StatusCancelled})
}

func (s *Server) HandleOrderStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/passenger/order_status/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing order_id")
		return
	}
	o, err := s.order.Get(r.Context(), types.ID(id))
	if err != nil {
		writeOrderError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"order_id": o.ID,
		"status":   o.Status,
	})
}
