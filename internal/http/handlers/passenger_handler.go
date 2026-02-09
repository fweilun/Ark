// README: Passenger handlers (order create/get).
package handlers

import (
    "encoding/json"
    "net/http"

    "ark/internal/modules/order"
    "ark/internal/types"
)

type PassengerHandler struct {
    order *order.Service
}

func NewPassengerHandler(orderSvc *order.Service) *PassengerHandler {
    return &PassengerHandler{order: orderSvc}
}

type requestRideReq struct {
    PassengerID string  `json:"passenger_id"`
    PickupLat   float64 `json:"pickup_lat"`
    PickupLng   float64 `json:"pickup_lng"`
    DropoffLat  float64 `json:"dropoff_lat"`
    DropoffLng  float64 `json:"dropoff_lng"`
    RideType    string  `json:"ride_type"`
}

func (h *PassengerHandler) RequestRide(w http.ResponseWriter, r *http.Request) {
    var req requestRideReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid json")
        return
    }
    if req.PassengerID == "" || req.RideType == "" {
        writeError(w, http.StatusBadRequest, "missing fields")
        return
    }
    id, err := h.order.Create(r.Context(), order.CreateCommand{
        PassengerID: types.ID(req.PassengerID),
        Pickup:      types.Point{Lat: req.PickupLat, Lng: req.PickupLng},
        Dropoff:     types.Point{Lat: req.DropoffLat, Lng: req.DropoffLng},
        RideType:    req.RideType,
    })
    if err != nil {
        writeOrderError(w, err)
        return
    }
    writeJSON(w, http.StatusCreated, map[string]any{"order_id": id, "status": order.StatusRequested})
}

func (h *PassengerHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if id == "" {
        writeError(w, http.StatusBadRequest, "missing order id")
        return
    }
    o, err := h.order.Get(r.Context(), types.ID(id))
    if err != nil {
        writeOrderError(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"order_id": o.ID, "status": o.Status})
}
