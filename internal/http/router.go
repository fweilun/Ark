// README: HTTP router registration (Gin).
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/http/handlers"
	"ark/internal/http/middleware"
	"ark/internal/modules/aiusage"
	"ark/internal/modules/location"
	"ark/internal/modules/matching"
	"ark/internal/modules/order"
	"ark/internal/modules/pricing"
)

// [TODO] We might want to check if the user have the permission to access the order
// [TODO] Satisfy minimum authentication principle.
func NewRouter(
	orderService *order.Service,
	matchingService *matching.Service,
	locationService *location.Service,
	pricingService *pricing.Service,
	aiService *aiusage.Service,
) *gin.Engine {
	// r := gin.New()
	// r.Use(middleware.Recovery())
	// r.Use(middleware.Logging())

	r := gin.Default()
	r.Use(middleware.Auth())

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

	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	return r
}
