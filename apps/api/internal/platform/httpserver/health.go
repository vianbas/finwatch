package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/vianbas/finwatch/apps/api/internal/platform/web"
)

// Pinger is the minimal dependency the readiness probe needs. *pgxpool.Pool
// satisfies it, and tests can supply a fake.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves liveness and readiness probes.
//
// Liveness answers "is the process running?" and never depends on external
// systems. Readiness answers "can the process serve traffic?" and checks
// downstream dependencies (currently PostgreSQL).
type HealthHandler struct {
	db          Pinger
	pingTimeout time.Duration
}

// NewHealthHandler builds a HealthHandler. db may be nil, in which case
// readiness reports that no database is configured.
func NewHealthHandler(db Pinger) *HealthHandler {
	return &HealthHandler{db: db, pingTimeout: 2 * time.Second}
}

// Live handles GET /health/live.
func (h *HealthHandler) Live(w http.ResponseWriter, _ *http.Request) {
	web.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready handles GET /health/ready. It returns 200 only when every checked
// dependency is reachable, otherwise 503.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		web.WriteError(w, http.StatusServiceUnavailable, "NOT_READY", "database not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.pingTimeout)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		web.WriteError(w, http.StatusServiceUnavailable, "NOT_READY", "database unreachable")
		return
	}
	web.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
