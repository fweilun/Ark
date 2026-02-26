// README: HTTP router registration (Gin).
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/http/handlers"
	"ark/internal/http/middleware"
	"ark/internal/modules/aiusage"
	"ark/internal/modules/calendar"
	"ark/internal/modules/location"
	"ark/internal/modules/matching"
	"ark/internal/modules/notification"
	"ark/internal/modules/order"
	"ark/internal/modules/pricing"
	"ark/internal/modules/user"
)

func NewRouter(
	orderService *order.Service,
	matchingService *matching.Service,
	locationService *location.Service,
	pricingService *pricing.Service,
	aiService *aiusage.Service,
	notificationService *notification.Service,
	calendarService *calendar.Service,
	userService *user.Service,
	tokenVerifier middleware.TokenVerifier,
) *gin.Engine {
	// r := gin.New()
	// r.Use(middleware.Recovery())
	// r.Use(middleware.Logging())

	r := gin.Default()

	// Public endpoints — no authentication required.
	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// All API routes require authentication.
	api := r.Group("/")
	api.Use(middleware.Auth(tokenVerifier))

	orderHandler := handlers.NewOrderHandler(orderService)
	// passenger — instant order
	api.POST("/api/orders", orderHandler.Create)
	api.GET("/api/orders/:id/status", orderHandler.Status)
	api.POST("/api/orders/:id/cancel", orderHandler.Cancel)
	// passenger — scheduled order
	api.POST("/api/orders/scheduled", orderHandler.CreateScheduled)
	api.GET("/api/orders/scheduled", orderHandler.ListScheduledByPassenger)
	api.GET("/api/orders/scheduled/available", orderHandler.ListAvailableScheduled)
	// driver — instant order
	api.POST("/api/orders/:id/match", orderHandler.Match)
	api.POST("/api/orders/:id/accept", orderHandler.Accept)
	api.POST("/api/orders/:id/deny", orderHandler.Deny)
	api.POST("/api/orders/:id/arrived", orderHandler.Arrive)
	api.POST("/api/orders/:id/meet", orderHandler.Meet)
	api.POST("/api/orders/:id/complete", orderHandler.Complete)
	api.POST("/api/orders/:id/pay", orderHandler.Pay)
	// driver — scheduled order
	api.POST("/api/orders/:id/claim", orderHandler.Claim)
	api.POST("/api/orders/:id/driver-cancel", orderHandler.DriverCancel)

	// ai model
	aiHandler := handlers.NewAIHandler(aiService)
	api.POST("/api/ai/chat", aiHandler.Chat)

	// location update
	locationHandler := handlers.NewLocationHandler(locationService)
	api.PUT("/api/drivers/:id/location", locationHandler.Update)

	// notifications
	notificationHandler := handlers.NewNotificationHandler(notificationService)
	api.POST("/api/notifications/register", notificationHandler.EnsureDevice)
	// [TODO] for staff only
	// api.POST("/api/notifications/send", notificationHandler.SendNotification)

	// calendar
	calendarHandler := handlers.NewCalendarHandler(calendarService)
	api.POST("/api/calendar/events", calendarHandler.CreateEvent)
	api.PUT("/api/calendar/events/:id", calendarHandler.EditEvent)
	api.DELETE("/api/calendar/events/:id", calendarHandler.DeleteEvent)
	api.POST("/api/calendar/schedules", calendarHandler.CreateAndTieOrder)
	api.DELETE("/api/calendar/schedules/:event_id/order", calendarHandler.UntieOrder)
	api.GET("/api/calendar/schedules", calendarHandler.ListSchedules)

	// users
	userHandler := handlers.NewUserHandler(userService)
	api.POST("/users", userHandler.Create)
	api.GET("/users/:id", userHandler.GetByID)
	api.GET("/me", userHandler.GetMe)
	api.PATCH("/users/:id", userHandler.UpdateName)
	api.DELETE("/users/:id", userHandler.Delete)

	return r
}
