// README: Order handlers for create/get/cancel.
package handlers

import (
    "encoding/json"
    "net/http"

    "ark/internal/modules/order"
    "ark/internal/types"
)

type OrderHandler struct {
    order *order.Service
}

func NewOrderHandler(svc *order.Service) *OrderHandler {
    return &OrderHandler{order: svc}
}

type createOrderReq struct {
    PassengerID string  `json:"passenger_id"`
    PickupLat   float64 `json:"pickup_lat"`
    PickupLng   float64 `json:"pickup_lng"`
    DropoffLat  float64 `json:"dropoff_lat"`
    DropoffLng  float64 `json:"dropoff_lng"`
    RideType    string  `json:"ride_type"`
}

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req createOrderReq
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
    writeJSON(w, http.StatusCreated, map[string]any{"order_id": id, "status": order.StatusCreated})
}

func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
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

func (h *OrderHandler) Cancel(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if id == "" {
        writeError(w, http.StatusBadRequest, "missing order id")
        return
    }
    err := h.order.Cancel(r.Context(), order.CancelCommand{
        OrderID:   types.ID(id),
        ActorType: "passenger",
        Reason:    "user_cancel",
    })
    if err != nil {
        writeOrderError(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"status": order.StatusCancelled})
}
