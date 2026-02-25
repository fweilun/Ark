// README: Health handler — checks infra connectivity and reports module status.
package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// HealthHandler checks infrastructure connectivity and reports which modules are loaded.
type HealthHandler struct {
	db      *pgxpool.Pool
	redis   *redis.Client
	modules []string
}

// NewHealthHandler creates a handler that pings db and redis, and lists the given module names.
func NewHealthHandler(db *pgxpool.Pool, rdb *redis.Client, modules []string) *HealthHandler {
	return &HealthHandler{db: db, redis: rdb, modules: modules}
}

type componentStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type healthResponse struct {
	Status    string                     `json:"status"`
	Timestamp string                     `json:"timestamp"`
	Checks    map[string]componentStatus `json:"checks"`
}

// Check handles GET /health.
// Returns 200 when all checks pass, 503 when any check fails.
func (h *HealthHandler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	checks := make(map[string]componentStatus, 2+len(h.modules))
	healthy := true

	// --- infrastructure checks ---
	checks["database"] = dbPing(ctx, h.db)
	if checks["database"].Status != "ok" {
		healthy = false
	}

	checks["redis"] = redisPing(ctx, h.redis)
	if checks["redis"].Status != "ok" {
		healthy = false
	}

	// --- module presence checks ---
	for _, name := range h.modules {
		checks["module:"+name] = componentStatus{Status: "ok"}
	}

	overall := "ok"
	if !healthy {
		overall = "degraded"
	}

	httpStatus := http.StatusOK
	if !healthy {
		httpStatus = http.StatusServiceUnavailable
	}

	writeJSON(c, httpStatus, healthResponse{
		Status:    overall,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	})
}

func dbPing(ctx context.Context, db *pgxpool.Pool) componentStatus {
	if db == nil {
		return componentStatus{Status: "unconfigured"}
	}
	if err := db.Ping(ctx); err != nil {
		return componentStatus{Status: "error", Error: err.Error()}
	}
	return componentStatus{Status: "ok"}
}

func redisPing(ctx context.Context, rdb *redis.Client) componentStatus {
	if rdb == nil {
		return componentStatus{Status: "unconfigured"}
	}
	if err := rdb.Ping(ctx).Err(); err != nil {
		return componentStatus{Status: "error", Error: err.Error()}
	}
	return componentStatus{Status: "ok"}
}
