package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RouterDeps are the dependencies required to build the HTTP router. As feature
// modules are added in later issues, their handlers are wired here.
type RouterDeps struct {
	Logger *slog.Logger
	Health *HealthHandler
}

// NewRouter builds the application's HTTP handler with the standard middleware
// chain (request ID, panic recovery, access logging) and baseline routes.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	r.Use(RequestID)
	r.Use(Recoverer(deps.Logger))
	r.Use(AccessLog(deps.Logger))

	// Operational endpoints. These are intentionally unauthenticated.
	r.Get("/health/live", deps.Health.Live)
	r.Get("/health/ready", deps.Health.Ready)

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})

	return r
}
