// README: Order handlers for create/get/cancel.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

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

func (h *OrderHandler) Create(c *gin.Context) {
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.PassengerID == "" || req.RideType == "" {
		writeError(c, http.StatusBadRequest, "missing fields")
		return
	}
	if !isValidID(req.PassengerID) {
		writeError(c, http.StatusBadRequest, "invalid passenger_id")
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

func (h *OrderHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		writeError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	o, err := h.order.Get(c.Request.Context(), types.ID(id))
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"order_id": o.ID, "status": o.Status})
}

func (h *OrderHandler) Cancel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		writeError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	err := h.order.Cancel(c.Request.Context(), order.CancelCommand{
		OrderID:   types.ID(id),
		ActorType: "passenger",
		Reason:    "user_cancel",
	})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusCancelled})
}

// Match is a temporary MVP endpoint to move order from created -> matched.
func (h *OrderHandler) Match(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		writeError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	driverID := c.Query("driver_id")
	if driverID == "" {
		writeError(c, http.StatusBadRequest, "missing driver_id")
		return
	}
	if !isValidID(driverID) {
		writeError(c, http.StatusBadRequest, "invalid driver_id")
		return
	}
	err := h.order.Match(c.Request.Context(), order.MatchCommand{
		OrderID:  types.ID(id),
		DriverID: types.ID(driverID),
	})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusDriverFound})
}

// Pay is a temporary MVP endpoint to move order from trip_complete -> payment.
func (h *OrderHandler) Pay(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		writeError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	err := h.order.Pay(c.Request.Context(), order.PayCommand{OrderID: types.ID(id)})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusPayment})
}
