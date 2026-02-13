// README: Passenger handlers (order create/get).
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"

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

func (h *PassengerHandler) RequestRide(c *gin.Context) {
    var req requestRideReq
    if err := c.ShouldBindJSON(&req); err != nil {
        writeError(c, http.StatusBadRequest, "invalid json")
        return
    }
    if req.PassengerID == "" || req.RideType == "" {
        writeError(c, http.StatusBadRequest, "missing fields")
        return
    }
    id, err := h.order.Create(c.Request.Context(), order.CreateCommand{
        PassengerID: types.ID(req.PassengerID),
        Pickup:      types.Point{Lat: req.PickupLat, Lng: req.PickupLng},
        Dropoff:     types.Point{Lat: req.DropoffLat, Lng: req.DropoffLng},
        RideType:    req.RideType,
    })
    if err != nil {
        writeOrderError(c, err)
        return
    }
    writeJSON(c, http.StatusCreated, map[string]any{"order_id": id, "status": order.StatusRequested})
}

func (h *PassengerHandler) GetOrder(c *gin.Context) {
    id := c.Param("id")
    if id == "" {
        writeError(c, http.StatusBadRequest, "missing order id")
        return
    }
    o, err := h.order.Get(c.Request.Context(), types.ID(id))
    if err != nil {
        writeOrderError(c, err)
        return
    }
    writeJSON(c, http.StatusOK, map[string]any{"order_id": o.ID, "status": o.Status})
}
