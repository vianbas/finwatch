package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vianbas/finwatch/apps/api/internal/platform/web"
)

// RouteRegistrar is implemented by feature modules to mount their routes on the
// application router. This keeps the platform layer independent of feature
// packages: cmd/api wires concrete modules in, the router only knows this port.
type RouteRegistrar interface {
	RegisterRoutes(r chi.Router)
}

// RouterDeps are the dependencies required to build the HTTP router.
type RouterDeps struct {
	Logger  *slog.Logger
	Health  *HealthHandler
	Modules []RouteRegistrar
}

// NewRouter builds the application's HTTP handler with the standard middleware
// chain (request ID, panic recovery, access logging), baseline operational
// routes, and any feature modules supplied in deps.Modules.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Use(RequestID)
	r.Use(Recoverer(deps.Logger))
	r.Use(AccessLog(deps.Logger))

	// Operational endpoints. These are intentionally unauthenticated.
	r.Get("/health/live", deps.Health.Live)
	r.Get("/health/ready", deps.Health.Ready)

	// Feature modules mount their own routes.
	for _, m := range deps.Modules {
		if m != nil {
			m.RegisterRoutes(r)
		}
	}

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		web.WriteError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		web.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})

	return r
}
