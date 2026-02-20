// README: Order handlers for create/get/cancel.
package handlers

import (
	"net/http"
	"time"

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
	writeJSON(c, http.StatusCreated, map[string]any{"order_id": id, "status": order.StatusWaiting})
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

func (h *OrderHandler) Status(c *gin.Context) {
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
	resp := map[string]any{
		"order_id":       o.ID,
		"status":         o.Status,
		"status_version": o.StatusVersion,
	}
	if o.DriverID != nil {
		resp["driver_id"] = *o.DriverID
	}
	writeJSON(c, http.StatusOK, resp)
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

// Match is a temporary MVP endpoint to move order from waiting -> approaching.
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
	writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusApproaching})
}

func (h *OrderHandler) Accept(c *gin.Context) {
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
	err := h.order.Accept(c.Request.Context(), order.AcceptCommand{
		OrderID:  types.ID(id),
		DriverID: types.ID(driverID),
	})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusApproaching})
}

func (h *OrderHandler) Deny(c *gin.Context) {
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
	err := h.order.Deny(c.Request.Context(), order.DenyCommand{
		OrderID:  types.ID(id),
		DriverID: types.ID(driverID),
	})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusDenied})
}

func (h *OrderHandler) Arrive(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		writeError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	err := h.order.Arrive(c.Request.Context(), order.ArriveCommand{OrderID: types.ID(id)})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusArrived})
}

func (h *OrderHandler) Meet(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		writeError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	err := h.order.Meet(c.Request.Context(), order.MeetCommand{OrderID: types.ID(id)})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusDriving})
}

func (h *OrderHandler) Complete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		writeError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	err := h.order.Complete(c.Request.Context(), order.CompleteCommand{OrderID: types.ID(id)})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusPayment})
}

// Pay is a temporary MVP endpoint to move order from payment -> complete.
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
	writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusComplete})
}

// --- Scheduled-order endpoints ---

type createScheduledReq struct {
	PassengerID        string  `json:"passenger_id"`
	PickupLat          float64 `json:"pickup_lat"`
	PickupLng          float64 `json:"pickup_lng"`
	DropoffLat         float64 `json:"dropoff_lat"`
	DropoffLng         float64 `json:"dropoff_lng"`
	RideType           string  `json:"ride_type"`
	ScheduledAt        string  `json:"scheduled_at"`        // RFC3339
	ScheduleWindowMins int     `json:"schedule_window_mins"` // minutes before scheduled_at to open for claiming
}

// CreateScheduled handles POST /api/orders/scheduled.
func (h *OrderHandler) CreateScheduled(c *gin.Context) {
	var req createScheduledReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.PassengerID == "" || req.RideType == "" || req.ScheduledAt == "" {
		writeError(c, http.StatusBadRequest, "missing fields")
		return
	}
	if !isValidID(req.PassengerID) {
		writeError(c, http.StatusBadRequest, "invalid passenger_id")
		return
	}
	if req.ScheduleWindowMins <= 0 {
		writeError(c, http.StatusBadRequest, "schedule_window_mins must be positive")
		return
	}
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid scheduled_at; expected RFC3339")
		return
	}
	id, err := h.order.CreateScheduled(c.Request.Context(), order.CreateScheduledCommand{
		PassengerID:        types.ID(req.PassengerID),
		Pickup:             types.Point{Lat: req.PickupLat, Lng: req.PickupLng},
		Dropoff:            types.Point{Lat: req.DropoffLat, Lng: req.DropoffLng},
		RideType:           req.RideType,
		ScheduledAt:        scheduledAt,
		ScheduleWindowMins: req.ScheduleWindowMins,
	})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, map[string]any{"order_id": id, "status": order.StatusScheduled})
}

// ListScheduledByPassenger handles GET /api/orders/scheduled?passenger_id=...
func (h *OrderHandler) ListScheduledByPassenger(c *gin.Context) {
	passengerID := c.Query("passenger_id")
	if passengerID == "" {
		writeError(c, http.StatusBadRequest, "missing passenger_id")
		return
	}
	if !isValidID(passengerID) {
		writeError(c, http.StatusBadRequest, "invalid passenger_id")
		return
	}
	orders, err := h.order.ListScheduledByPassenger(c.Request.Context(), types.ID(passengerID))
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"orders": orders})
}

// ListAvailableScheduled handles GET /api/orders/scheduled/available?from=...&to=...
func (h *OrderHandler) ListAvailableScheduled(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		writeError(c, http.StatusBadRequest, "missing from or to")
		return
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid from; expected RFC3339")
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid to; expected RFC3339")
		return
	}
	orders, err := h.order.ListAvailableScheduled(c.Request.Context(), from, to)
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"orders": orders})
}

type claimReq struct {
	DriverID string `json:"driver_id"`
}

// Claim handles POST /api/orders/:id/claim (driver claims a scheduled order).
func (h *OrderHandler) Claim(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		writeError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	var req claimReq
	if err := c.ShouldBindJSON(&req); err != nil || req.DriverID == "" {
		writeError(c, http.StatusBadRequest, "missing driver_id")
		return
	}
	if !isValidID(req.DriverID) {
		writeError(c, http.StatusBadRequest, "invalid driver_id")
		return
	}
	err := h.order.ClaimScheduled(c.Request.Context(), order.ClaimScheduledCommand{
		OrderID:  types.ID(id),
		DriverID: types.ID(req.DriverID),
	})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusAssigned})
}

type driverCancelReq struct {
	DriverID string `json:"driver_id"`
	Reason   string `json:"reason"`
}

// DriverCancel handles POST /api/orders/:id/driver-cancel (driver cancels a claimed scheduled order).
func (h *OrderHandler) DriverCancel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		writeError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	var req driverCancelReq
	if err := c.ShouldBindJSON(&req); err != nil || req.DriverID == "" {
		writeError(c, http.StatusBadRequest, "missing driver_id")
		return
	}
	if !isValidID(req.DriverID) {
		writeError(c, http.StatusBadRequest, "invalid driver_id")
		return
	}
	err := h.order.CancelScheduledByDriver(c.Request.Context(), order.DriverCancelScheduledCommand{
		OrderID:  types.ID(id),
		DriverID: types.ID(req.DriverID),
	})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusScheduled})
}
