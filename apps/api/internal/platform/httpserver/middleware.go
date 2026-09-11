package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/vianbas/finwatch/apps/api/internal/platform/web"
)

// contextKey is an unexported type to avoid collisions in request context.
type contextKey string

const requestIDKey contextKey = "request_id"

// requestIDHeader is the canonical header used to propagate a request ID across
// service and proxy boundaries.
const requestIDHeader = "X-Request-ID"

// RequestID assigns a stable identifier to each request. An inbound
// X-Request-ID is trusted and echoed; otherwise a new random ID is generated.
// The ID is stored on the context and mirrored on the response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the request ID associated with ctx, or "".
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func newRequestID() string {
	b := make([]byte, 16)
	// crypto/rand.Read never returns an error on supported platforms; if it
	// somehow does, fall back to an empty-but-valid identifier rather than panic.
	if _, err := rand.Read(b); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}

// Recoverer converts a panic in a downstream handler into a structured log line
// and a 500 response, preventing a single bad request from crashing the server.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.ErrorContext(r.Context(), "panic recovered",
						slog.Any("panic", rec),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.String("request_id", RequestIDFromContext(r.Context())),
					)
					web.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder captures the response status code for access logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// CORS returns a middleware that answers cross-origin requests from an
// explicit allow-list of origins, using exact string comparison (no
// wildcards, no suffix matching). It exists to let the web app (served from a
// different origin than the API) complete a JSON POST /login and send
// requests carrying an Authorization header without the browser blocking
// them.
//
// Requests with no Origin header pass through with no CORS headers added at
// all. A request that does carry an Origin header always gets Vary: Origin,
// even when that origin is not on the allow-list, so that shared caches never
// reuse a response computed for one origin when serving another. A preflight
// request (method OPTIONS with an Access-Control-Request-Method header) from
// an allowed origin is answered directly with 204 and is never forwarded to
// next. Any other request from an allowed origin is additionally annotated
// with Access-Control-Allow-Origin before being forwarded.
//
// Access-Control-Allow-Credentials is never set: tokens travel in the
// Authorization header, not cookies.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Add("Vary", "Origin")
			if !allowed[origin] {
				next.ServeHTTP(w, r)
				return
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Methods", "GET, POST")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			next.ServeHTTP(w, r)
		})
	}
}

// AccessLog emits one structured JSON line per request with method, path,
// status and latency. It is the only request-scoped logging in the skeleton.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			logger.InfoContext(r.Context(), "http_request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", RequestIDFromContext(r.Context())),
			)
		})
	}
}
