// README: HTTP handlers for the calendar module.
package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ark/internal/http/dto"
	"ark/internal/http/middleware"
	"ark/internal/httpx"
	"ark/internal/modules/calendar"
	"ark/internal/modules/order"
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

// --- request types ---

type createEventReq struct {
	From        string `json:"from" binding:"required"`  // RFC3339
	To          string `json:"to" binding:"required"`    // RFC3339
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

type editEventReq struct {
	From        string `json:"from" binding:"required"`  // RFC3339
	To          string `json:"to" binding:"required"`    // RFC3339
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

type createAndTieOrderReq struct {
	EventID    string  `json:"event_id" binding:"required"`
	PickupLat  float64 `json:"pickup_lat"`
	PickupLng  float64 `json:"pickup_lng"`
	DropoffLat float64 `json:"dropoff_lat"`
	DropoffLng float64 `json:"dropoff_lng"`
	RideType   string  `json:"ride_type" binding:"required"`
}

// schedulesListResponse wraps a list of schedules. Keeps the existing
// {"schedules": [...]} shape while each element is a flat schedule DTO.
type schedulesListResponse struct {
	Schedules []dto.CalendarScheduleResponse `json:"schedules"`
}

// CreateEvent handles POST /api/calendar/events.
//
// @Summary      Create calendar event
// @Description  Creates a calendar event for the authenticated user. `from`/`to` must be RFC3339 timestamps.
// @Tags         Calendar
// @Accept       json
// @Produce      json
// @Security     FirebaseAuth
// @Param        body  body      createEventReq               true  "Event from/to/title/description"
// @Success      201   {object}  dto.CalendarEventResponse
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Router       /api/calendar/events [post]
func (h *CalendarHandler) CreateEvent(c *gin.Context) {
	var req createEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	from, err := time.Parse(time.RFC3339, req.From)
	if err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid from; expected RFC3339")
		return
	}
	to, err := time.Parse(time.RFC3339, req.To)
	if err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid to; expected RFC3339")
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
	writeJSON(c, http.StatusCreated, dto.CalendarEventResponse{EventID: string(id)})
}

// EditEvent handles PUT /api/calendar/events/:id.
//
// @Summary      Edit calendar event
// @Description  Replaces the stored calendar event with the supplied fields. Same validation rules as CreateEvent.
// @Tags         Calendar
// @Accept       json
// @Produce      json
// @Security     FirebaseAuth
// @Param        id    path      string                        true  "Event ID"
// @Param        body  body      editEventReq                  true  "Updated event fields"
// @Success      200   {object}  dto.CalendarEventResponse
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Failure      404   {object}  httpx.ErrorBody
// @Router       /api/calendar/events/{id} [put]
func (h *CalendarHandler) EditEvent(c *gin.Context) {
	id := c.Param("id")
	if id == "" || !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid event id")
		return
	}
	var req editEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	from, err := time.Parse(time.RFC3339, req.From)
	if err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid from; expected RFC3339")
		return
	}
	to, err := time.Parse(time.RFC3339, req.To)
	if err != nil {
		httpx.RespondError(c, http.StatusBadRequest, "invalid to; expected RFC3339")
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
	writeJSON(c, http.StatusOK, dto.CalendarEventResponse{EventID: id})
}

// DeleteEvent handles DELETE /api/calendar/events/:id.
//
// @Summary      Delete calendar event
// @Description  Removes the calendar event (and any ride schedule tied to it). Returns 204 on success.
// @Tags         Calendar
// @Security     FirebaseAuth
// @Param        id   path      string              true  "Event ID"
// @Success      204
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      404  {object}  httpx.ErrorBody
// @Router       /api/calendar/events/{id} [delete]
func (h *CalendarHandler) DeleteEvent(c *gin.Context) {
	id := c.Param("id")
	if id == "" || !isValidID(id) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid event id")
		return
	}
	if err := h.svc.DeleteEvent(c.Request.Context(), types.ID(id)); err != nil {
		writeCalendarError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// CreateAndTieOrder handles POST /api/calendar/schedules — creates a ride order and ties it to an existing event.
// The authenticated user_id (from context) is used as both the schedule UID and the passenger_id.
//
// @Summary      Tie a ride order to a calendar event
// @Description  Creates a scheduled ride order whose pickup window is derived from the referenced calendar event, and ties the two together so cancelling/deleting the event also untethers the order.
// @Tags         Calendar
// @Accept       json
// @Produce      json
// @Security     FirebaseAuth
// @Param        body  body      createAndTieOrderReq          true  "Event id + ride pickup/dropoff"
// @Success      201   {object}  dto.CalendarScheduleResponse
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      401   {object}  httpx.ErrorBody
// @Failure      404   {object}  httpx.ErrorBody
// @Failure      409   {object}  httpx.ErrorBody
// @Router       /api/calendar/schedules [post]
func (h *CalendarHandler) CreateAndTieOrder(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req createAndTieOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !isValidID(req.EventID) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid event_id")
		return
	}
	sc, err := h.svc.CreateAndTieOrder(c.Request.Context(), calendar.CreateAndTieOrderCommand{
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
	writeJSON(c, http.StatusCreated, scheduleToDTO(sc))
}

// UntieOrder handles DELETE /api/calendar/schedules/:event_id/order.
// The authenticated user_id from context is used as the schedule UID.
//
// @Summary      Untie ride order from calendar event
// @Description  Breaks the tie between a calendar event and its associated scheduled order. The event stays; the order is cancelled.
// @Tags         Calendar
// @Security     FirebaseAuth
// @Param        event_id  path  string  true  "Calendar event ID"
// @Success      204
// @Failure      400  {object}  httpx.ErrorBody
// @Failure      401  {object}  httpx.ErrorBody
// @Failure      404  {object}  httpx.ErrorBody
// @Router       /api/calendar/schedules/{event_id}/order [delete]
func (h *CalendarHandler) UntieOrder(c *gin.Context) {
	eventID := c.Param("event_id")
	if eventID == "" || !isValidID(eventID) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid event id")
		return
	}
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.svc.UntieOrder(c.Request.Context(), calendar.UntieOrderCommand{
		UID:     types.ID(userID),
		EventID: types.ID(eventID),
	}); err != nil {
		writeCalendarError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListSchedules handles GET /api/calendar/schedules.
// The authenticated user_id from context is used to filter schedules.
//
// @Summary      List schedules for current user
// @Description  Returns every calendar-backed ride schedule the authenticated user owns, flattened into the `schedules` array.
// @Tags         Calendar
// @Produce      json
// @Security     FirebaseAuth
// @Success      200  {object}  schedulesListResponse
// @Failure      401  {object}  httpx.ErrorBody
// @Router       /api/calendar/schedules [get]
func (h *CalendarHandler) ListSchedules(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c.Request.Context())
	if !ok {
		httpx.RespondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	schedules, err := h.svc.ListSchedulesByUser(c.Request.Context(), types.ID(userID))
	if err != nil {
		writeCalendarError(c, err)
		return
	}
	out := make([]dto.CalendarScheduleResponse, 0, len(schedules))
	for _, sc := range schedules {
		out = append(out, scheduleToDTO(sc))
	}
	writeJSON(c, http.StatusOK, schedulesListResponse{Schedules: out})
}

// scheduleToDTO maps an internal *calendar.Schedule to the flat
// CalendarScheduleResponse wire shape.
func scheduleToDTO(sc *calendar.Schedule) dto.CalendarScheduleResponse {
	resp := dto.CalendarScheduleResponse{
		UID:     string(sc.UID),
		EventID: string(sc.EventID),
	}
	if sc.TiedOrder != nil {
		id := string(*sc.TiedOrder)
		resp.TiedOrder = &id
	}
	return resp
}

func writeCalendarError(c *gin.Context, err error) {
	switch err {
	case calendar.ErrBadRequest, order.ErrBadRequest, order.ErrActiveOrder:
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
	case calendar.ErrNotFound, order.ErrNotFound:
		httpx.RespondError(c, http.StatusNotFound, err.Error())
	case order.ErrInvalidState, order.ErrConflict:
		httpx.RespondError(c, http.StatusConflict, err.Error())
	default:
		httpx.RespondError(c, http.StatusInternalServerError, "internal error")
	}
}
