package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func registrar(register func(chi.Router)) RouteRegistrar {
	return RegistrarFunc(register)
}

func TestRouter_PublicModuleRunsWithoutAuthMiddleware(t *testing.T) {
	h := NewHealthHandler(fakePinger{})
	router := NewRouter(RouterDeps{
		Logger: testLogger(),
		Health: h,
		PublicModules: []RouteRegistrar{
			registrar(func(r chi.Router) {
				r.Get("/public", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
			}),
		},
		RequireAuth: func(http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized) // would reject everything if applied
			})
		},
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/public", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (public route must bypass RequireAuth)", rec.Code)
	}
}

func TestRouter_ProtectedModuleRunsBehindAuthMiddleware(t *testing.T) {
	h := NewHealthHandler(fakePinger{})
	router := NewRouter(RouterDeps{
		Logger: testLogger(),
		Health: h,
		Modules: []RouteRegistrar{
			registrar(func(r chi.Router) {
				r.Get("/protected", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
			}),
		},
		RequireAuth: func(http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			})
		},
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/protected", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (protected route must run behind RequireAuth)", rec.Code)
	}
}
