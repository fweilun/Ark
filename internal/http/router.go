// README: HTTP router registration (Gin).
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/http/handlers"
	"ark/internal/http/middleware"
	"ark/internal/modules/location"
	"ark/internal/modules/matching"
	"ark/internal/modules/order"
	"ark/internal/modules/pricing"
)

func NewRouter(
	orderService *order.Service,
	matchingService *matching.Service,
	locationService *location.Service,
	pricingService *pricing.Service,
) *gin.Engine {
	r := gin.Default()
	// r := gin.New()
	// r.Use(middleware.Recovery())
	// r.Use(middleware.Logging())
	r.Use(middleware.Auth())

	orderHandler := handlers.NewOrderHandler(orderService)
	r.POST("/api/orders", orderHandler.Create)
	r.GET("/api/orders/:id", orderHandler.Get)
	r.GET("/api/orders/:id/status", orderHandler.Status)
	r.POST("/api/orders/:id/match", orderHandler.Match)
	r.POST("/api/orders/:id/accept", orderHandler.Accept)
	r.POST("/api/orders/:id/deny", orderHandler.Deny)
	r.POST("/api/orders/:id/arrived", orderHandler.Arrive)
	r.POST("/api/orders/:id/meet", orderHandler.Meet)
	r.POST("/api/orders/:id/complete", orderHandler.Complete)
	r.POST("/api/orders/:id/cancel", orderHandler.Cancel)
	r.POST("/api/orders/:id/pay", orderHandler.Pay)

	driverHandler := handlers.NewDriverHandler(orderService, matchingService)
	r.GET("/api/drivers/:id/orders", driverHandler.ListAvailable)
	r.GET("/api/drivers/orders", driverHandler.ListAvailable)
	r.POST("/api/drivers/orders/:id/accept", driverHandler.Accept)
	r.POST("/api/drivers/orders/:id/start", driverHandler.Start)
	r.POST("/api/drivers/orders/:id/complete", driverHandler.Complete)
	r.POST("/api/drivers/orders/:id/deny", driverHandler.Deny)

	locationHandler := handlers.NewLocationHandler(locationService)
	r.PUT("/api/drivers/:id/location", locationHandler.Update)

	passengerHandler := handlers.NewPassengerHandler(orderService)
	r.POST("/api/passengers/orders", passengerHandler.RequestRide)
	r.GET("/api/passengers/orders/:id", passengerHandler.GetOrder)

	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	return r
}
