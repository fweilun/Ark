// README: Ride assistant service — orchestrates AI parsing, session state, and order creation.
package rideassistant

import (
	"context"
	"fmt"
	"log"
	"time"

	"ark/internal/types"
)

// OrderCreator is the subset of order.Service needed to create rides.
type OrderCreator interface {
	Create(ctx context.Context, cmd CreateOrderCommand) (types.ID, error)
	CreateScheduled(ctx context.Context, cmd CreateScheduledOrderCommand) (types.ID, error)
}

// CreateOrderCommand mirrors order.CreateCommand to avoid circular imports.
type CreateOrderCommand struct {
	PassengerID types.ID
	Pickup      types.Point
	Dropoff     types.Point
	RideType    string
}

// CreateScheduledOrderCommand mirrors order.CreateScheduledCommand.
type CreateScheduledOrderCommand struct {
	PassengerID        types.ID
	Pickup             types.Point
	Dropoff            types.Point
	RideType           string
	ScheduledAt        time.Time
	ScheduleWindowMins int
}

// Planner is the AI planner/parser interface.
type Planner interface {
	Parse(ctx context.Context, req ParserRequest) (*ParserResponse, error)
}

// Geocoder converts a text address into latitude/longitude coordinates.
type Geocoder interface {
	Geocode(ctx context.Context, address string) (lat, lng float64, err error)
}

// Service is the main ride assistant service.
type Service struct {
	store    *Store
	planner  Planner
	orders   OrderCreator // nil until order integration is wired
	geocoder Geocoder     // nil if geocoding is not available
	loc      *time.Location
}

// NewService creates a ride assistant service.
func NewService(store *Store, planner Planner, orders OrderCreator, geocoder Geocoder) *Service {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		loc = time.UTC
	}
	return &Service{
		store:    store,
		planner:  planner,
		orders:   orders,
		geocoder: geocoder,
		loc:      loc,
	}
}

// HandleMessage is the main entry point for processing a user message.
// It follows a synchronous flow: get/create session → call AI → merge → respond.
func (s *Service) HandleMessage(ctx context.Context, userID string, req MessageRequest) (*MessageResponse, error) {
	// 1. Get or create session.
	sess, err := s.getOrCreateSession(userID, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("session lookup: %w", err)
	}

	// 2. Call AI parser with timeout.
	aiCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	parserReq := s.buildParserRequest(sess, req)
	parsed, err := s.planner.Parse(aiCtx, parserReq)
	if err != nil {
		return nil, fmt.Errorf("ai planner: %w", err)
	}

	// 3. Handle cancellation intent.
	if parsed.Intent == "cancel" {
		_ = s.store.CancelSession(sess.ID)
		return &MessageResponse{
			Status:  "cancelled",
			Reply:   parsed.Reply,
			Session: NewSessionView(sess),
		}, nil
	}

	// 4. Handle non-booking chat.
	if parsed.Intent == "chat" {
		return &MessageResponse{
			Status:  "chat",
			Reply:   parsed.Reply,
			Session: NewSessionView(sess),
		}, nil
	}

	// 5. Merge AI output into session.
	s.mergeSession(sess, parsed)
	s.store.UpdateSession(sess)

	// 6. Decide response.
	return s.buildResponse(ctx, sess, parsed)
}

// ---------------------------------------------------------------------------
// Session helpers
// ---------------------------------------------------------------------------

func (s *Service) getOrCreateSession(userID, requestedID string) (*Session, error) {
	// If a specific session ID was requested, try to load it.
	if requestedID != "" {
		sess, err := s.store.GetSession(requestedID)
		if err != nil {
			return nil, err
		}
		if sess != nil {
			return sess, nil
		}
		// Session not found or expired — fall through to create new.
	}

	// Try existing active session for this user.
	sess, err := s.store.GetActiveSessionByUserID(userID)
	if err != nil {
		return nil, err
	}
	if sess != nil {
		return sess, nil
	}

	// Create new session.
	return s.store.CreateSession(userID), nil
}

// ---------------------------------------------------------------------------
// AI integration
// ---------------------------------------------------------------------------

func (s *Service) buildParserRequest(sess *Session, req MessageRequest) ParserRequest {
	state := map[string]string{
		"stage": sess.Stage,
	}
	if sess.PickupText != "" {
		state["pickup_text"] = sess.PickupText
	}
	if sess.DropoffText != "" {
		state["dropoff_text"] = sess.DropoffText
	}
	if sess.DepartureAt != nil {
		state["departure_at"] = sess.DepartureAt.Format(time.RFC3339)
	}
	if sess.PendingQuestion != "" {
		state["pending_question"] = sess.PendingQuestion
	}

	return ParserRequest{
		UserMessage:  req.Message,
		SessionState: state,
		ContextInfo:  req.ContextInfo,
	}
}

// ---------------------------------------------------------------------------
// Merge AI results into session
// ---------------------------------------------------------------------------

func (s *Service) mergeSession(sess *Session, parsed *ParserResponse) {
	if parsed.PickupText != nil && *parsed.PickupText != "" {
		sess.PickupText = *parsed.PickupText
	}
	if parsed.DropoffText != nil && *parsed.DropoffText != "" {
		sess.DropoffText = *parsed.DropoffText
	}
	if parsed.DepartureAt != nil && *parsed.DepartureAt != "" {
		if t, err := time.Parse(time.RFC3339, *parsed.DepartureAt); err == nil {
			t = t.In(s.loc)
			sess.DepartureAt = &t
		}
	}

	// Update pending question and summary from AI reply.
	if len(parsed.MissingFields) > 0 {
		sess.PendingQuestion = parsed.Reply
	} else {
		sess.PendingQuestion = ""
	}

	if parsed.NeedsConfirmation {
		sess.Stage = StageConfirming
		sess.Summary = parsed.Reply
	}
}

// ---------------------------------------------------------------------------
// Response builder
// ---------------------------------------------------------------------------

func (s *Service) buildResponse(ctx context.Context, sess *Session, parsed *ParserResponse) (*MessageResponse, error) {
	view := NewSessionView(sess)

	// Validation before booking: departure must be in the future.
	if parsed.ReadyToBook && sess.DepartureAt != nil {
		if sess.DepartureAt.Before(time.Now()) {
			return &MessageResponse{
				Status:  "clarification",
				Reply:   "您指定的出發時間已經過了，請提供一個未來的時間。",
				Session: view,
			}, nil
		}
	}

	// Ready to book — create order.
	if parsed.ReadyToBook && sess.AllFieldsPresent() {
		booking, err := s.createBooking(ctx, sess)
		if err != nil {
			log.Printf("rideassistant: booking failed for session %s: %v", sess.ID, err)
			return &MessageResponse{
				Status:  "clarification",
				Reply:   "抱歉，建立訂單時發生錯誤，請稍後再試。",
				Session: view,
			}, nil
		}
		_ = s.store.CompleteSession(sess.ID)
		view.Stage = StageCompleted
		return &MessageResponse{
			Status:  "completed",
			Reply:   parsed.Reply,
			Session: view,
			Booking: booking,
		}, nil
	}

	// Needs confirmation.
	if parsed.NeedsConfirmation && sess.AllFieldsPresent() {
		return &MessageResponse{
			Status:  "confirmation",
			Reply:   parsed.Reply,
			Session: view,
		}, nil
	}

	// Still collecting — clarification needed.
	return &MessageResponse{
		Status:  "clarification",
		Reply:   parsed.Reply,
		Session: view,
	}, nil
}

// ---------------------------------------------------------------------------
// Order creation
// ---------------------------------------------------------------------------

func (s *Service) createBooking(ctx context.Context, sess *Session) (*BookingResult, error) {
	if s.orders == nil {
		return &BookingResult{
			OrderID: "stub_" + sess.ID,
			Status:  "created",
		}, nil
	}

	// Geocode pickup and dropoff addresses.
	pickup, err := s.geocodeAddress(ctx, sess.PickupText)
	if err != nil {
		return nil, fmt.Errorf("geocode pickup %q: %w", sess.PickupText, err)
	}
	dropoff, err := s.geocodeAddress(ctx, sess.DropoffText)
	if err != nil {
		return nil, fmt.Errorf("geocode dropoff %q: %w", sess.DropoffText, err)
	}

	// Determine if this is a scheduled or instant ride.
	isScheduled := sess.DepartureAt != nil && time.Until(*sess.DepartureAt) > 5*time.Minute
	sess.IsScheduled = isScheduled

	userID := types.ID(sess.UserID)

	if isScheduled {
		orderID, err := s.orders.CreateScheduled(ctx, CreateScheduledOrderCommand{
			PassengerID:        userID,
			Pickup:             pickup,
			Dropoff:            dropoff,
			RideType:           "standard",
			ScheduledAt:        *sess.DepartureAt,
			ScheduleWindowMins: 15,
		})
		if err != nil {
			return nil, err
		}
		return &BookingResult{OrderID: string(orderID), Status: "scheduled"}, nil
	}

	orderID, err := s.orders.Create(ctx, CreateOrderCommand{
		PassengerID: userID,
		Pickup:      pickup,
		Dropoff:     dropoff,
		RideType:    "standard",
	})
	if err != nil {
		return nil, err
	}
	return &BookingResult{OrderID: string(orderID), Status: "waiting"}, nil
}

func (s *Service) geocodeAddress(ctx context.Context, address string) (types.Point, error) {
	if s.geocoder == nil {
		return types.Point{}, nil
	}
	lat, lng, err := s.geocoder.Geocode(ctx, address)
	if err != nil {
		return types.Point{}, err
	}
	return types.Point{Lat: lat, Lng: lng}, nil
}
