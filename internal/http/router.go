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

// [TODO] We might want to check if the user have the permission to access the order
// [TODO] Satisfy minimum authentication principle.
func NewRouter(
	orderService *order.Service,
	matchingService *matching.Service,
	locationService *location.Service,
	pricingService *pricing.Service,
) *gin.Engine {
	// r := gin.New()
	// r.Use(middleware.Recovery())
	// r.Use(middleware.Logging())

	r := gin.Default()
	r.Use(middleware.Auth())

	orderHandler := handlers.NewOrderHandler(orderService)
	// passenger
	r.POST("/api/orders", orderHandler.Create)
	r.GET("/api/orders/:id/status", orderHandler.Status)
	r.POST("/api/orders/:id/cancel", orderHandler.Cancel)
	// driver
	// r.GET("/api/orders/:id/status", orderHandler.Status)
	r.POST("/api/orders/:id/match", orderHandler.Match)
	r.POST("/api/orders/:id/accept", orderHandler.Accept)
	r.POST("/api/orders/:id/deny", orderHandler.Deny)
	r.POST("/api/orders/:id/arrived", orderHandler.Arrive)
	r.POST("/api/orders/:id/meet", orderHandler.Meet)
	r.POST("/api/orders/:id/cancel", orderHandler.Cancel)
	r.POST("/api/orders/:id/pay", orderHandler.Pay)

	// location udpate
	locationHandler := handlers.NewLocationHandler(locationService)
	r.PUT("/api/drivers/:id/location", locationHandler.Update)

	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	return r
}
