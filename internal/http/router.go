// README: HTTP router registration (Gin).
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "ark/docs" // swagger generated docs — side-effect import registers SwaggerInfo
	"ark/internal/http/dto"
	"ark/internal/http/handlers"
	"ark/internal/http/middleware"
	"ark/internal/modules/aiusage"
	"ark/internal/modules/calendar"
	"ark/internal/modules/driver"
	"ark/internal/modules/location"
	"ark/internal/modules/matching"
	"ark/internal/modules/notification"
	"ark/internal/modules/order"
	"ark/internal/modules/pricing"
	"ark/internal/modules/relation"
	"ark/internal/modules/rideassistant"
	"ark/internal/modules/user"
	"ark/internal/worker"
)

// readyzProbeTimeout caps the total time /readyz spends pinging
// downstream dependencies, so a hung dependency does not itself hang
// the probe. Kept short because liveness/readiness probes are called
// frequently and must fail fast.
const readyzProbeTimeout = 2 * time.Second

// apiVersion is returned by GET /health. Hardcoded for now — bump on each release.
const apiVersion = "0.1.0"

func NewRouter(
	orderService *order.Service,
	matchingService *matching.Service,
	locationService *location.Service,
	pricingService *pricing.Service,
	aiService *aiusage.Service,
	notificationService *notification.Service,
	calendarService *calendar.Service,
	driverService *driver.Service,
	userService *user.Service,
	relationService *relation.Service,
	tokenVerifier middleware.TokenVerifier,
	rideAssistantSvc *rideassistant.Service,
	dbPool *pgxpool.Pool,
	redisClient *redis.Client,
	workerRegistry *worker.Registry,
) *gin.Engine {
	// r := gin.New()
	// r.Use(middleware.Recovery())
	// r.Use(middleware.Logging())

	r := gin.Default()

	// Public endpoints — no authentication required.
	// Health returns a simple {"status": "ok", "version": "..."} payload.
	// Detailed component checks (db, redis, workers) have moved off this
	// endpoint; liveness probes should stay cheap.
	r.GET("/health", healthHandler)

	// /readyz is the readiness probe. It pings Postgres and Redis with a
	// short timeout and returns 200 {status:"ok"} when every dependency is
	// reachable, or 503 {status:"degraded", detail:<concise error>} the
	// moment any one of them fails. The probe never blocks longer than
	// readyzProbeTimeout so a hung downstream cannot wedge the endpoint.
	r.GET("/readyz", readyzHandler(dbPool, redisClient))

	// Swagger UI — public, serves the generated OpenAPI docs under /swagger/*.
	// The docs package is registered via the side-effect import above; Host is
	// overridden at startup from PUBLIC_HOST (see cmd/ark-api/main.go).
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
	// POST /api/users is auth-required: the handler reads the verified Firebase
	// UID from the token and uses it as the new user's primary key, so calls
	// that land here without a valid token could not produce a row that
	// GET /api/me (also keyed by UID) would later find.
	api.POST("/api/users", userHandler.CreateUser)
	api.GET("/api/me", userHandler.GetMe)
	api.PATCH("/api/me", userHandler.UpdateMe)
	api.DELETE("/api/me", userHandler.DeleteMe)

	// driver profile & status (auth required; driver_id always from context)
	driverHandler := handlers.NewDriverHandler(driverService)
	api.POST("/api/driver/create", driverHandler.Create)
	api.PATCH("/api/driver/status", driverHandler.UpdateStatus)

	// relations (friend requests & friendships)
	relationHandler := handlers.NewRelationHandler(relationService)
	api.POST("/api/relations/requests", relationHandler.SendRequest)
	api.POST("/api/relations/requests/by-phone", relationHandler.SendRequestByPhone)
	api.GET("/api/relations/search", relationHandler.SearchUsers)
	api.GET("/api/relations/requests/received", relationHandler.ListReceived)
	api.GET("/api/relations/requests/sent", relationHandler.ListSent)
	api.DELETE("/api/relations/requests/:friend_id", relationHandler.CancelRequest)
	api.POST("/api/relations/requests/:friend_id/accept", relationHandler.AcceptRequest)
	api.POST("/api/relations/requests/:friend_id/reject", relationHandler.RejectRequest)
	api.GET("/api/relations/friends", relationHandler.ListFriends)
	api.DELETE("/api/relations/friends/:friend_id", relationHandler.RemoveFriend)
	api.GET("/api/relations/friends/:friend_id/is", relationHandler.IsFriend)

	// ride assistant
	if rideAssistantSvc != nil {
		raHandler := handlers.NewRideAssistantHandler(rideAssistantSvc)
		api.POST("/api/assistant/ride/messages", raHandler.HandleMessage)
	}

	return r
}

// readyzDetailResponse is the 503 body emitted by /readyz when a
// dependency probe fails.
type readyzDetailResponse struct {
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// readyzOKResponse is the 200 body emitted by /readyz when every
// probed dependency is reachable.
type readyzOKResponse struct {
	Status string `json:"status"`
}

// healthHandler is the liveness probe.
//
// @Summary      Liveness probe
// @Description  Returns {status:"ok", version:"<api version>"}. Never touches downstream dependencies so the probe stays cheap.
// @Tags         Health
// @Produce      json
// @Success      200  {object}  dto.HealthResponse
// @Router       /health [get]
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, dto.HealthResponse{Status: "ok", Version: apiVersion})
}

// readyzHandler returns a Gin handler that pings Postgres and Redis
// under a short timeout and reports degraded/ok. The closure captures
// the pool/client so the endpoint-level signature stays handler-shaped
// (gin.HandlerFunc) and swag can annotate the returned handler.
//
// @Summary      Readiness probe
// @Description  Pings Postgres and Redis with a 2s timeout; returns 200 {status:"ok"} when both are healthy, 503 {status:"degraded", detail:"<component>: <error>"} on any failure.
// @Tags         Health
// @Produce      json
// @Success      200  {object}  readyzOKResponse
// @Failure      503  {object}  readyzDetailResponse
// @Router       /readyz [get]
func readyzHandler(dbPool *pgxpool.Pool, redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readyzProbeTimeout)
		defer cancel()
		if dbPool != nil {
			if err := dbPool.Ping(ctx); err != nil {
				c.JSON(http.StatusServiceUnavailable, readyzDetailResponse{
					Status: "degraded",
					Detail: "postgres: " + err.Error(),
				})
				return
			}
		}
		if redisClient != nil {
			if err := redisClient.Ping(ctx).Err(); err != nil {
				c.JSON(http.StatusServiceUnavailable, readyzDetailResponse{
					Status: "degraded",
					Detail: "redis: " + err.Error(),
				})
				return
			}
		}
		c.JSON(http.StatusOK, readyzOKResponse{Status: "ok"})
	}
}
