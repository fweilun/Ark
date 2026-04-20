// README: Order handlers for create/get/cancel.
package handlers

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ark/internal/http/dto"
	"ark/internal/http/middleware"
	"ark/internal/httpx"
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
	PickupLat  float64 `json:"pickup_lat"`
	PickupLng  float64 `json:"pickup_lng"`
	DropoffLat float64 `json:"dropoff_lat"`
	DropoffLng float64 `json:"dropoff_lng"`
	RideType   string  `json:"ride_type" binding:"required"`
}

// Create handles POST /api/orders — passenger creates an instant (realtime) order.
//
// @Summary      Create instant order
// @Description  Creates an on-demand (immediate) ride order for the authenticated passenger and enters the waiting queue.
// @Tags         Order
// @Accept       json
// @Produce      json
// @Security     FirebaseAuth
// @Param        body  body      createOrderReq      true  "Pickup/dropoff and ride type"
// @Success      201   {object}  dto.OrderResponse
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Failure      409   {object}  httpx.ErrorBody
// @Router       /api/orders [post]
func (h *OrderHandler) Create(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req createOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	id, err := h.order.Create(c.Request.Context(), order.CreateCommand{
		PassengerID: types.ID(userID),
		Pickup:      types.Point{Lat: req.PickupLat, Lng: req.PickupLng},
		Dropoff:     types.Point{Lat: req.DropoffLat, Lng: req.DropoffLng},
		RideType:    req.RideType,
	})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, dto.OrderResponse{
		OrderID: string(id),
		Status:  string(order.StatusWaiting),
	})
}

// Get handles GET /api/orders/:id — returns the order's minimal summary.
// Note: not currently registered on the router; kept for future use.
func (h *OrderHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	o, err := h.order.Get(c.Request.Context(), types.ID(id))
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, dto.OrderResponse{
		OrderID: string(o.ID),
		Status:  string(o.Status),
	})
}

// Status handles GET /api/orders/:id/status — returns the order's current status,
// status_version (optimistic-lock version), and assigned driver_id (if any).
//
// @Summary      Get order status
// @Description  Returns the current status, optimistic-lock version, and assigned driver_id (if any) for a single order.
// @Tags         Order
// @Produce      json
// @Security     FirebaseAuth
// @Param        id   path      string                    true  "Order ID"
// @Success      200  {object}  dto.OrderStatusResponse
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      404  {object}  httpx.ErrorBody
// @Router       /api/orders/{id}/status [get]
func (h *OrderHandler) Status(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	o, err := h.order.Get(c.Request.Context(), types.ID(id))
	if err != nil {
		writeOrderError(c, err)
		return
	}
	resp := dto.OrderStatusResponse{
		OrderID:       string(o.ID),
		Status:        string(o.Status),
		StatusVersion: o.StatusVersion,
	}
	if o.DriverID != nil {
		resp.DriverID = string(*o.DriverID)
	}
	writeJSON(c, http.StatusOK, resp)
}

// Cancel handles POST /api/orders/:id/cancel — passenger cancels an order.
//
// @Summary      Cancel order (passenger)
// @Description  Cancels the order as the passenger. For scheduled orders past the free-cancel deadline, `late_cancel:true` is returned so the client can warn the user that a penalty may apply.
// @Tags         Order
// @Produce      json
// @Security     FirebaseAuth
// @Param        id   path      string              true  "Order ID"
// @Success      200  {object}  dto.OrderResponse
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      404  {object}  httpx.ErrorBody
// @Failure      409  {object}  httpx.ErrorBody
// @Router       /api/orders/{id}/cancel [post]
func (h *OrderHandler) Cancel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid order id")
		return
	}

	// Check before cancellation whether this is a scheduled order past its free-cancel deadline.
	// The order is still cancelled (MVP), but we inform the client so they can show the appropriate message.
	lateCancel := false
	if o, err := h.order.Get(c.Request.Context(), types.ID(id)); err == nil {
		if o.OrderType == "scheduled" && o.CancelDeadlineAt != nil && time.Now().After(*o.CancelDeadlineAt) {
			lateCancel = true
		}
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
	writeJSON(c, http.StatusOK, dto.OrderResponse{
		Status:     string(order.StatusCancelled),
		LateCancel: &lateCancel,
	})
}

// Match is a temporary MVP endpoint to move order from waiting -> approaching.
//
// @Summary      Match order to driver (MVP)
// @Description  Temporary MVP endpoint that atomically matches an order in `waiting` with the authenticated driver and transitions it to `approaching`. Will be replaced by the dispatcher.
// @Tags         Order
// @Produce      json
// @Security     FirebaseAuth
// @Param        id   path      string              true  "Order ID"
// @Success      200  {object}  dto.OrderResponse
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      409  {object}  httpx.ErrorBody
// @Router       /api/orders/{id}/match [post]
func (h *OrderHandler) Match(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	driverID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
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
	writeJSON(c, http.StatusOK, dto.OrderResponse{Status: string(order.StatusApproaching)})
}

// Accept handles POST /api/orders/:id/accept — driver accepts an order.
//
// @Summary      Accept order (driver)
// @Description  Driver accepts a ride offer; transitions the order to `approaching`.
// @Tags         Order
// @Produce      json
// @Security     FirebaseAuth
// @Param        id   path      string              true  "Order ID"
// @Success      200  {object}  dto.OrderResponse
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      409  {object}  httpx.ErrorBody
// @Router       /api/orders/{id}/accept [post]
func (h *OrderHandler) Accept(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	driverID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
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
	writeJSON(c, http.StatusOK, dto.OrderResponse{Status: string(order.StatusApproaching)})
}

// Deny handles POST /api/orders/:id/deny — driver declines an order; it falls back to `waiting`.
//
// @Summary      Deny order (driver)
// @Description  Driver declines the ride offer. The order is returned to the `waiting` queue for re-matching.
// @Tags         Order
// @Produce      json
// @Security     FirebaseAuth
// @Param        id   path      string              true  "Order ID"
// @Success      200  {object}  dto.OrderResponse
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      409  {object}  httpx.ErrorBody
// @Router       /api/orders/{id}/deny [post]
func (h *OrderHandler) Deny(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	driverID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
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
	// [CHECK]
	writeJSON(c, http.StatusOK, dto.OrderResponse{Status: string(order.StatusWaiting)})
}

// Arrive handles POST /api/orders/:id/arrived — driver signals they have reached the pickup point.
//
// @Summary      Driver arrived at pickup
// @Description  Driver signals arrival at the pickup location. Transitions the order from `approaching` to `arrived`.
// @Tags         Order
// @Produce      json
// @Security     FirebaseAuth
// @Param        id   path      string              true  "Order ID"
// @Success      200  {object}  dto.OrderResponse
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      409  {object}  httpx.ErrorBody
// @Router       /api/orders/{id}/arrived [post]
func (h *OrderHandler) Arrive(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	err := h.order.Arrive(c.Request.Context(), order.ArriveCommand{OrderID: types.ID(id)})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, dto.OrderResponse{Status: string(order.StatusArrived)})
}

// Meet handles POST /api/orders/:id/meet — passenger has boarded; the trip begins.
//
// @Summary      Passenger boarded
// @Description  Passenger has boarded; transitions the order from `arrived` to `driving` and the trip begins.
// @Tags         Order
// @Produce      json
// @Security     FirebaseAuth
// @Param        id   path      string              true  "Order ID"
// @Success      200  {object}  dto.OrderResponse
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      409  {object}  httpx.ErrorBody
// @Router       /api/orders/{id}/meet [post]
func (h *OrderHandler) Meet(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	err := h.order.Meet(c.Request.Context(), order.MeetCommand{OrderID: types.ID(id)})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, dto.OrderResponse{Status: string(order.StatusDriving)})
}

// Complete handles POST /api/orders/:id/complete — driver signals trip completion; order moves into `payment`.
//
// @Summary      Trip complete (enter payment)
// @Description  Driver signals dropoff is complete; transitions the order from `driving` to `payment` so the passenger can settle the fare.
// @Tags         Order
// @Produce      json
// @Security     FirebaseAuth
// @Param        id   path      string              true  "Order ID"
// @Success      200  {object}  dto.OrderResponse
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      409  {object}  httpx.ErrorBody
// @Router       /api/orders/{id}/complete [post]
func (h *OrderHandler) Complete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	err := h.order.Complete(c.Request.Context(), order.CompleteCommand{OrderID: types.ID(id)})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, dto.OrderResponse{Status: string(order.StatusPayment)})
}

// Pay is a temporary MVP endpoint to move order from payment -> complete.
//
// @Summary      Pay for trip (MVP)
// @Description  Temporary MVP endpoint that marks the order as paid and transitions it from `payment` to `complete`. Will be replaced by the real payment flow.
// @Tags         Order
// @Produce      json
// @Security     FirebaseAuth
// @Param        id   path      string              true  "Order ID"
// @Success      200  {object}  dto.OrderResponse
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      409  {object}  httpx.ErrorBody
// @Router       /api/orders/{id}/pay [post]
func (h *OrderHandler) Pay(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	err := h.order.Pay(c.Request.Context(), order.PayCommand{OrderID: types.ID(id)})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, dto.OrderResponse{Status: string(order.StatusComplete)})
}

// --- Scheduled-order endpoints ---

type createScheduledReq struct {
	PickupLat          float64 `json:"pickup_lat"`
	PickupLng          float64 `json:"pickup_lng"`
	DropoffLat         float64 `json:"dropoff_lat"`
	DropoffLng         float64 `json:"dropoff_lng"`
	RideType           string  `json:"ride_type" binding:"required"`
	ScheduledAt        string  `json:"scheduled_at" binding:"required"` // RFC3339
	ScheduleWindowMins int     `json:"schedule_window_mins" binding:"required,gt=0"`
}

// CreateScheduled handles POST /api/orders/scheduled.
//
// @Summary      Create scheduled order
// @Description  Creates a scheduled (future) ride. The driver pool is surfaced via the available-scheduled endpoint; drivers claim it within the schedule window.
// @Tags         Order
// @Accept       json
// @Produce      json
// @Security     FirebaseAuth
// @Param        body  body      createScheduledReq  true  "Pickup/dropoff, ride_type, RFC3339 scheduled_at and schedule_window_mins"
// @Success      201   {object}  dto.OrderResponse
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Router       /api/orders/scheduled [post]
func (h *OrderHandler) CreateScheduled(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req createScheduledReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid scheduled_at; expected RFC3339")
		return
	}
	id, err := h.order.CreateScheduled(c.Request.Context(), order.CreateScheduledCommand{
		PassengerID:        types.ID(userID),
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
	writeJSON(c, http.StatusCreated, dto.OrderResponse{
		OrderID: string(id),
		Status:  string(order.StatusScheduled),
	})
}

// scheduledOrdersResponse wraps a list of scheduled-order summaries for
// the passenger and available-order endpoints. Each element is a
// dto.ScheduledOrderSummary (per-field projection of order.Order) instead
// of the domain struct, so the HTTP layer owns the wire shape and domain
// renames cannot leak into the API response.
type scheduledOrdersResponse struct {
	Orders []dto.ScheduledOrderSummary `json:"orders"`
}

// orderToScheduledSummary projects an *order.Order onto the public
// ScheduledOrderSummary DTO, preserving every field the underlying model
// exposes via json tags. Keeping this as an explicit per-field mapper (as
// opposed to embedding order.Order in the DTO) stops future additions to
// order.Order from silently appearing in the API response.
func orderToScheduledSummary(o *order.Order) dto.ScheduledOrderSummary {
	if o == nil {
		return dto.ScheduledOrderSummary{}
	}
	sum := dto.ScheduledOrderSummary{
		ID:                 string(o.ID),
		PassengerID:        string(o.PassengerID),
		Status:             string(o.Status),
		StatusVersion:      o.StatusVersion,
		Pickup:             o.Pickup,
		Dropoff:            o.Dropoff,
		RideType:           o.RideType,
		EstimatedFee:       o.EstimatedFee,
		ActualFee:          o.ActualFee,
		CreatedAt:          o.CreatedAt,
		MatchedAt:          o.MatchedAt,
		AcceptedAt:         o.AcceptedAt,
		StartedAt:          o.StartedAt,
		CompletedAt:        o.CompletedAt,
		CancelledAt:        o.CancelledAt,
		CancelReason:       o.CancelReason,
		OrderType:          o.OrderType,
		ScheduledAt:        o.ScheduledAt,
		ScheduleWindowMins: o.ScheduleWindowMins,
		CancelDeadlineAt:   o.CancelDeadlineAt,
		IncentiveBonus:     o.IncentiveBonus,
		AssignedAt:         o.AssignedAt,
	}
	if o.DriverID != nil {
		id := string(*o.DriverID)
		sum.DriverID = &id
	}
	return sum
}

// orderListToScheduledSummaries maps a slice of domain orders to a slice
// of summaries (never nil, so the JSON response always serialises to `[]`
// instead of `null`).
func orderListToScheduledSummaries(orders []*order.Order) []dto.ScheduledOrderSummary {
	out := make([]dto.ScheduledOrderSummary, 0, len(orders))
	for _, o := range orders {
		out = append(out, orderToScheduledSummary(o))
	}
	return out
}

// ListScheduledByPassenger handles GET /api/orders/scheduled.
//
// @Summary      List my scheduled orders
// @Description  Returns every scheduled order owned by the authenticated passenger, projected through ScheduledOrderSummary.
// @Tags         Order
// @Produce      json
// @Security     FirebaseAuth
// @Success      200  {object}  scheduledOrdersResponse
// @Failure      401  {object}  httpx.ErrorBody
// @Router       /api/orders/scheduled [get]
func (h *OrderHandler) ListScheduledByPassenger(c *gin.Context) {
	passengerID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	orders, err := h.order.ListScheduledByPassenger(c.Request.Context(), types.ID(passengerID))
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, scheduledOrdersResponse{Orders: orderListToScheduledSummaries(orders)})
}

// ListAvailableScheduled handles GET /api/orders/scheduled/available?from=...&to=...
//
// @Summary      List claimable scheduled orders
// @Description  Returns scheduled orders within [from,to] that have not yet been claimed by a driver. Both `from` and `to` must be RFC3339 timestamps and `from` must be strictly before `to`.
// @Tags         Order
// @Produce      json
// @Security     FirebaseAuth
// @Param        from  query     string  true  "RFC3339 window start (inclusive)"
// @Param        to    query     string  true  "RFC3339 window end (exclusive)"
// @Success      200   {object}  scheduledOrdersResponse
// @Failure      400   {object}  httpx.ErrorBody
// @Router       /api/orders/scheduled/available [get]
func (h *OrderHandler) ListAvailableScheduled(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing from or to")
		return
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid from; expected RFC3339")
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid to; expected RFC3339")
		return
	}
	if !from.Before(to) {
		httpx.RespondError(c, http.StatusBadRequest, "from must be before to")
		return
	}
	orders, err := h.order.ListAvailableScheduled(c.Request.Context(), from, to)
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, scheduledOrdersResponse{Orders: orderListToScheduledSummaries(orders)})
}

// Claim handles POST /api/orders/:id/claim (driver claims a scheduled order).
//
// @Summary      Claim scheduled order (driver)
// @Description  Driver claims a `scheduled` order; transitions it to `assigned` and binds the driver_id until the driver departs or cancels.
// @Tags         Order
// @Produce      json
// @Security     FirebaseAuth
// @Param        id   path      string              true  "Order ID"
// @Success      200  {object}  dto.OrderResponse
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      409  {object}  httpx.ErrorBody
// @Router       /api/orders/{id}/claim [post]
func (h *OrderHandler) Claim(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	driverID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	err := h.order.ClaimScheduled(c.Request.Context(), order.ClaimScheduledCommand{
		OrderID:  types.ID(id),
		DriverID: types.ID(driverID),
	})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, dto.OrderResponse{Status: string(order.StatusAssigned)})
}

type driverCancelReq struct {
	Reason string `json:"reason"`
}

// DriverCancel handles POST /api/orders/:id/driver-cancel (driver cancels a claimed scheduled order).
//
// @Summary      Release claimed scheduled order (driver)
// @Description  Driver releases a previously-claimed `assigned` scheduled order back to `scheduled` so another driver can pick it up. Optional `reason` is recorded on the order history.
// @Tags         Order
// @Accept       json
// @Produce      json
// @Security     FirebaseAuth
// @Param        id    path      string               true   "Order ID"
// @Param        body  body      driverCancelReq      false  "Optional reason string"
// @Success      200   {object}  dto.OrderResponse
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Failure      409   {object}  httpx.ErrorBody
// @Router       /api/orders/{id}/driver-cancel [post]
func (h *OrderHandler) DriverCancel(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing order id")
		return
	}
	if !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid order id")
		return
	}
	driverID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req driverCancelReq
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	err := h.order.CancelScheduledByDriver(c.Request.Context(), order.DriverCancelScheduledCommand{
		OrderID:  types.ID(id),
		DriverID: types.ID(driverID),
		Reason:   req.Reason,
	})
	if err != nil {
		writeOrderError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, dto.OrderResponse{Status: string(order.StatusScheduled)})
}
