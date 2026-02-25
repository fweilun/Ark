// README: HTTP router registration (Gin).
package http

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"ark/internal/http/handlers"
	"ark/internal/http/middleware"
	"ark/internal/modules/aiusage"
	"ark/internal/modules/calendar"
	"ark/internal/modules/location"
	"ark/internal/modules/matching"
	"ark/internal/modules/notification"
	"ark/internal/modules/order"
	"ark/internal/modules/pricing"
)

// [TODO] We might want to check if the user have the permission to access the order
// [TODO] Satisfy minimum authentication principle.
func NewRouter(
	db *pgxpool.Pool,
	rdb *redis.Client,
	orderService *order.Service,
	matchingService *matching.Service,
	locationService *location.Service,
	pricingService *pricing.Service,
	aiService *aiusage.Service,
	notificationService *notification.Service,
	calendarService *calendar.Service,
) *gin.Engine {
	// r := gin.New()
	// r.Use(middleware.Recovery())
	// r.Use(middleware.Logging())

	r := gin.Default()
	r.Use(middleware.Auth())

	healthHandler := handlers.NewHealthHandler(db, rdb, []string{
		"order", "matching", "location", "pricing", "ai", "notification", "calendar",
	})
	r.GET("/health", healthHandler.Check)

	orderHandler := handlers.NewOrderHandler(orderService)
	// passenger — instant order
	r.POST("/api/orders", orderHandler.Create)
	r.GET("/api/orders/:id/status", orderHandler.Status)
	r.POST("/api/orders/:id/cancel", orderHandler.Cancel)
	// passenger — scheduled order
	r.POST("/api/orders/scheduled", orderHandler.CreateScheduled)
	r.GET("/api/orders/scheduled", orderHandler.ListScheduledByPassenger)
	r.GET("/api/orders/scheduled/available", orderHandler.ListAvailableScheduled)
	// driver — instant order
	r.POST("/api/orders/:id/match", orderHandler.Match)
	r.POST("/api/orders/:id/accept", orderHandler.Accept)
	r.POST("/api/orders/:id/deny", orderHandler.Deny)
	r.POST("/api/orders/:id/arrived", orderHandler.Arrive)
	r.POST("/api/orders/:id/meet", orderHandler.Meet)
	r.POST("/api/orders/:id/complete", orderHandler.Complete)
	r.POST("/api/orders/:id/pay", orderHandler.Pay)
	// driver — scheduled order
	r.POST("/api/orders/:id/claim", orderHandler.Claim)
	r.POST("/api/orders/:id/driver-cancel", orderHandler.DriverCancel)

	// ai model
	aiHandler := handlers.NewAIHandler(aiService)
	r.POST("/api/ai/chat", aiHandler.Chat)

	// location udpate
	locationHandler := handlers.NewLocationHandler(locationService)
	r.PUT("/api/drivers/:id/location", locationHandler.Update)

	// notifications
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	r.POST("/api/notifications/register", notificationHandler.EnsureDevice)
	// [TODO] for staff only
	// r.POST("/api/notifications/send", notificationHandler.SendNotification)

	// calendar
	calendarHandler := handlers.NewCalendarHandler(calendarService)
	r.POST("/api/calendar/events", calendarHandler.CreateEvent)
	r.PUT("/api/calendar/events/:id", calendarHandler.EditEvent)
	r.DELETE("/api/calendar/events/:id", calendarHandler.DeleteEvent)
	r.POST("/api/calendar/schedules", calendarHandler.CreateAndTieOrder)
	r.DELETE("/api/calendar/schedules/:event_id/order", calendarHandler.UntieOrder)
	r.GET("/api/calendar/schedules", calendarHandler.ListSchedules)

	return r
}
