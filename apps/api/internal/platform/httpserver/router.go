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

// RegistrarFunc adapts a plain function to RouteRegistrar, the same pattern
// as http.HandlerFunc.
type RegistrarFunc func(r chi.Router)

// RegisterRoutes implements RouteRegistrar.
func (f RegistrarFunc) RegisterRoutes(r chi.Router) { f(r) }

// RouterDeps are the dependencies required to build the HTTP router.
type RouterDeps struct {
	Logger *slog.Logger
	Health *HealthHandler
	// PublicModules mount routes that do not require authentication (e.g. login).
	PublicModules []RouteRegistrar
	// Modules mount routes that require a valid bearer token when RequireAuth
	// is set.
	Modules []RouteRegistrar
	// RequireAuth, if non-nil, wraps Modules' routes. It is a plain middleware
	// function so this package has no dependency on the auth feature package.
	RequireAuth func(http.Handler) http.Handler
	// CORSAllowedOrigins is the exact-match allow-list for cross-origin
	// browser requests (e.g. the web app's origin). When empty or nil, the
	// CORS middleware is not applied.
	CORSAllowedOrigins []string
}

// NewRouter builds the application's HTTP handler with the standard middleware
// chain (request ID, panic recovery, access logging), baseline operational
// routes, and any feature modules supplied in deps.Modules.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Use(RequestID)
	r.Use(Recoverer(deps.Logger))
	r.Use(AccessLog(deps.Logger))
	if len(deps.CORSAllowedOrigins) > 0 {
		r.Use(CORS(deps.CORSAllowedOrigins))
	}

	// Operational endpoints. These are intentionally unauthenticated.
	r.Get("/health/live", deps.Health.Live)
	r.Get("/health/ready", deps.Health.Ready)

	// Public feature routes (e.g. login) mount without auth middleware.
	for _, m := range deps.PublicModules {
		if m != nil {
			m.RegisterRoutes(r)
		}
	}

	// Protected feature routes run behind RequireAuth when configured.
	r.Group(func(pr chi.Router) {
		if deps.RequireAuth != nil {
			pr.Use(deps.RequireAuth)
		}
		for _, m := range deps.Modules {
			if m != nil {
				m.RegisterRoutes(pr)
			}
		}
	})

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		web.WriteError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		web.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})

	return r
}
