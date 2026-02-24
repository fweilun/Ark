// README: HTTP handlers for the calendar module.
package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ark/internal/modules/calendar"
	"ark/internal/types"
)

// CalendarHandler exposes calendar event and schedule endpoints.
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

type createAndTieOrderReq struct {
	UID         string `json:"uid"`
	From        string `json:"from"`        // RFC3339
	To          string `json:"to"`          // RFC3339
	Title       string `json:"title"`
	Description string `json:"description"`
	OrderID     string `json:"order_id"`
}

// CreateEvent handles POST /api/calendar/events.
func (h *CalendarHandler) CreateEvent(c *gin.Context) {
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

// CreateAndTieOrder handles POST /api/calendar/schedules — creates an event and ties it to an order.
func (h *CalendarHandler) CreateAndTieOrder(c *gin.Context) {
	var req createAndTieOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.UID == "" || req.OrderID == "" || req.From == "" || req.To == "" || req.Title == "" {
		writeError(c, http.StatusBadRequest, "missing fields")
		return
	}
	if !isValidID(req.UID) {
		writeError(c, http.StatusBadRequest, "invalid uid")
		return
	}
	if !isValidID(req.OrderID) {
		writeError(c, http.StatusBadRequest, "invalid order_id")
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
	sc, err := h.svc.CreateAndTieOrder(c.Request.Context(), calendar.CreateAndTieOrderCommand{
		UID:         types.ID(req.UID),
		From:        from,
		To:          to,
		Title:       req.Title,
		Description: req.Description,
		OrderID:     types.ID(req.OrderID),
	})
	if err != nil {
		writeCalendarError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, map[string]any{
		"uid":        sc.UID,
		"event_id":   sc.EventID,
		"tied_order": sc.TiedOrder,
	})
}

// UntieOrder handles DELETE /api/calendar/schedules/:event_id/order?uid=...
func (h *CalendarHandler) UntieOrder(c *gin.Context) {
	eventID := c.Param("event_id")
	if eventID == "" || !isValidID(eventID) {
		writeError(c, http.StatusBadRequest, "invalid event id")
		return
	}
	uid := c.Query("uid")
	if uid == "" || !isValidID(uid) {
		writeError(c, http.StatusBadRequest, "invalid uid")
		return
	}
	if err := h.svc.UntieOrder(c.Request.Context(), calendar.UntieOrderCommand{
		UID:     types.ID(uid),
		EventID: types.ID(eventID),
	}); err != nil {
		writeCalendarError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListSchedules handles GET /api/calendar/schedules?uid=...
func (h *CalendarHandler) ListSchedules(c *gin.Context) {
	uid := c.Query("uid")
	if uid == "" || !isValidID(uid) {
		writeError(c, http.StatusBadRequest, "invalid uid")
		return
	}
	schedules, err := h.svc.ListSchedulesByUser(c.Request.Context(), types.ID(uid))
	if err != nil {
		writeCalendarError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"schedules": schedules})
}

func writeCalendarError(c *gin.Context, err error) {
	switch err {
	case calendar.ErrBadRequest:
		writeError(c, http.StatusBadRequest, err.Error())
	case calendar.ErrNotFound:
		writeError(c, http.StatusNotFound, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal error")
	}
}
