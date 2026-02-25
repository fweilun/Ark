// README: HTTP server wiring (Gin engine factory).
package http

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"ark/internal/modules/aiusage"
	"ark/internal/modules/calendar"
	"ark/internal/modules/location"
	"ark/internal/modules/matching"
	"ark/internal/modules/notification"
	"ark/internal/modules/order"
	"ark/internal/modules/pricing"
)

type ServerDeps struct {
	DB           *pgxpool.Pool
	Redis        *redis.Client
	Order        *order.Service
	Matching     *matching.Service
	Location     *location.Service
	Pricing      *pricing.Service
	AI           *aiusage.Service
	Notification *notification.Service
	Calendar     *calendar.Service
}

type Server struct {
	Engine http.Handler
}

func NewServer(deps ServerDeps) *Server {
	engine := NewRouter(deps.DB, deps.Redis, deps.Order, deps.Matching, deps.Location, deps.Pricing, deps.AI, deps.Notification, deps.Calendar)
	return &Server{Engine: engine}
}

func (s *Server) Routes() http.Handler {
	return s.Engine
}
