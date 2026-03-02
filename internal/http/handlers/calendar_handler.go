// README: HTTP handlers for the calendar module.
package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ark/internal/http/middleware"
	"ark/internal/modules/calendar"
	"ark/internal/modules/order"
	"ark/internal/types"
)

// CalendarHandler exposes calendar event and order-event endpoints.
type CalendarHandler struct {
	svc *calendar.Service
}

// NewCalendarHandler creates a CalendarHandler backed by the given service.
func NewCalendarHandler(svc *calendar.Service) *CalendarHandler {
	return &CalendarHandler{svc: svc}
}

// --- request/response types ---

type createEventReq struct {
	From        string `json:"from"`        // RFC3339
	To          string `json:"to"`          // RFC3339
	Title       string `json:"title"`
	Description string `json:"description"`
}

type editEventReq struct {
	From        string `json:"from"`        // RFC3339
	To          string `json:"to"`          // RFC3339
	Title       string `json:"title"`
	Description string `json:"description"`
}

type createOrderEventReq struct {
	EventID    string  `json:"event_id"`
	PickupLat  float64 `json:"pickup_lat"`
	PickupLng  float64 `json:"pickup_lng"`
	DropoffLat float64 `json:"dropoff_lat"`
	DropoffLng float64 `json:"dropoff_lng"`
	RideType   string  `json:"ride_type"`
}

// CreateEvent handles POST /api/calendar/events.
// The authenticated user is automatically registered as an attendee.
func (h *CalendarHandler) CreateEvent(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req createEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.From == "" || req.To == "" || req.Title == "" {
		writeError(c, http.StatusBadRequest, "missing fields")
		return
	}
	from, err := time.Parse(time.RFC3339, req.From)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid from; expected RFC3339")
		return
	}
	to, err := time.Parse(time.RFC3339, req.To)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid to; expected RFC3339")
		return
	}
	id, err := h.svc.CreateEvent(c.Request.Context(), calendar.CreateEventCommand{
		UID:         types.ID(userID),
		From:        from,
		To:          to,
		Title:       req.Title,
		Description: req.Description,
	})
	if err != nil {
		writeCalendarError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, map[string]any{"event_id": id})
}

// EditEvent handles PUT /api/calendar/events/:id.
func (h *CalendarHandler) EditEvent(c *gin.Context) {
	id := c.Param("id")
	if id == "" || !isValidID(id) {
		writeError(c, http.StatusBadRequest, "invalid event id")
		return
	}
	var req editEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.From == "" || req.To == "" || req.Title == "" {
		writeError(c, http.StatusBadRequest, "missing fields")
		return
	}
	from, err := time.Parse(time.RFC3339, req.From)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid from; expected RFC3339")
		return
	}
	to, err := time.Parse(time.RFC3339, req.To)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid to; expected RFC3339")
		return
	}
	if err := h.svc.EditEvent(c.Request.Context(), calendar.EditEventCommand{
		ID:          types.ID(id),
		From:        from,
		To:          to,
		Title:       req.Title,
		Description: req.Description,
	}); err != nil {
		writeCalendarError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"event_id": id})
}

// DeleteEvent handles DELETE /api/calendar/events/:id.
func (h *CalendarHandler) DeleteEvent(c *gin.Context) {
	id := c.Param("id")
	if id == "" || !isValidID(id) {
		writeError(c, http.StatusBadRequest, "invalid event id")
		return
	}
	if err := h.svc.DeleteEvent(c.Request.Context(), types.ID(id)); err != nil {
		writeCalendarError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// CreateOrderEvent handles POST /api/calendar/order-events.
// Creates a ride order and links it to an existing calendar event.
// The authenticated user is used as both the schedule owner and the passenger.
func (h *CalendarHandler) CreateOrderEvent(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req createOrderEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.EventID == "" || req.RideType == "" {
		writeError(c, http.StatusBadRequest, "missing fields")
		return
	}
	if !isValidID(req.EventID) {
		writeError(c, http.StatusBadRequest, "invalid event_id")
		return
	}
	oe, err := h.svc.CreateOrderEvent(c.Request.Context(), calendar.CreateOrderEventCommand{
		UID:         types.ID(userID),
		EventID:     types.ID(req.EventID),
		PassengerID: types.ID(userID),
		Pickup:      types.Point{Lat: req.PickupLat, Lng: req.PickupLng},
		Dropoff:     types.Point{Lat: req.DropoffLat, Lng: req.DropoffLng},
		RideType:    req.RideType,
	})
	if err != nil {
		writeCalendarError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, map[string]any{
		"id":       oe.ID,
		"event_id": oe.EventID,
		"order_id": oe.OrderID,
		"uid":      oe.UID,
	})
}

// CancelOrderEvent handles DELETE /api/calendar/order-events/:id.
// Cancels the linked ride order and removes the order-event link.
func (h *CalendarHandler) CancelOrderEvent(c *gin.Context) {
	orderEventID := c.Param("id")
	if orderEventID == "" || !isValidID(orderEventID) {
		writeError(c, http.StatusBadRequest, "invalid order event id")
		return
	}
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.svc.CancelOrderEvent(c.Request.Context(), calendar.CancelOrderEventCommand{
		UID:          types.ID(userID),
		OrderEventID: types.ID(orderEventID),
	}); err != nil {
		writeCalendarError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListAllEvents handles GET /api/calendar/events.
// Returns all calendar events for the authenticated user.
func (h *CalendarHandler) ListAllEvents(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	events, err := h.svc.ListAllEvents(c.Request.Context(), types.ID(userID))
	if err != nil {
		writeCalendarError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"events": events})
}

// ListAllOrders handles GET /api/calendar/order-events.
// Returns all order-event links for the authenticated user.
func (h *CalendarHandler) ListAllOrders(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	orderEvents, err := h.svc.ListAllOrders(c.Request.Context(), types.ID(userID))
	if err != nil {
		writeCalendarError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"order_events": orderEvents})
}

func writeCalendarError(c *gin.Context, err error) {
	switch err {
	case calendar.ErrBadRequest, order.ErrBadRequest, order.ErrActiveOrder:
		writeError(c, http.StatusBadRequest, err.Error())
	case calendar.ErrNotFound, order.ErrNotFound:
		writeError(c, http.StatusNotFound, err.Error())
	case calendar.ErrForbidden:
		writeError(c, http.StatusForbidden, err.Error())
	case order.ErrInvalidState, order.ErrConflict:
		writeError(c, http.StatusConflict, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal error")
	}
}
