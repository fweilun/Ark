// README: HTTP server wiring (Gin engine factory).
package http

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"ark/internal/http/middleware"
	"ark/internal/worker"
	"ark/internal/modules/aiusage"
	"ark/internal/modules/rideassistant"
	"ark/internal/modules/calendar"
	"ark/internal/modules/driver"
	"ark/internal/modules/location"
	"ark/internal/modules/matching"
	"ark/internal/modules/notification"
	"ark/internal/modules/order"
	"ark/internal/modules/pricing"
	"ark/internal/modules/relation"
	"ark/internal/modules/user"
)

type ServerDeps struct {
	Order        *order.Service
	Matching     *matching.Service
	Location     *location.Service
	Pricing      *pricing.Service
	AI           aiusage.Service
	Notification *notification.Service
	Calendar     *calendar.Service
	Driver       *driver.Service
	User         *user.Service
	Relation     *relation.Service
	Auth         middleware.TokenVerifier // Firebase token verifier; nil disables auth (dev mode)
	RideAssistant *rideassistant.Service
	DB            *pgxpool.Pool
	Redis         *redis.Client
	Workers       *worker.Registry
}

type Server struct {
	Engine http.Handler
}

func NewServer(deps ServerDeps) *Server {
	engine := NewRouter(deps.Order, deps.Matching, deps.Location, deps.Pricing, deps.AI, deps.Notification, deps.Calendar, deps.Driver, deps.User, deps.Relation, deps.Auth, deps.RideAssistant, deps.DB, deps.Redis, deps.Workers)
	return &Server{Engine: engine}
}

func (s *Server) Routes() http.Handler {
	return s.Engine
}
