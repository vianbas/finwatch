package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestCORS_PreflightFromAllowedOrigin(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS([]string{"http://localhost:5173"})(next)

	req := httptest.NewRequest(http.MethodOptions, "/login", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if nextCalled {
		t.Error("next handler was called for a preflight request, want it skipped")
	}

	h := rec.Header()
	if got := h.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:5173")
	}
	if got := h.Get("Access-Control-Allow-Methods"); got != "GET, POST" {
		t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, "GET, POST")
	}
	if got := h.Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type" {
		t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, "Authorization, Content-Type")
	}
	if got := h.Get("Access-Control-Max-Age"); got != "600" {
		t.Errorf("Access-Control-Max-Age = %q, want %q", got, "600")
	}
	if got := h.Values("Vary"); len(got) != 1 || got[0] != "Origin" {
		t.Errorf("Vary = %v, want [Origin]", got)
	}
	if got := h.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want unset", got)
	}
}

func TestCORS_PreflightFromDisallowedOrigin(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS([]string{"http://localhost:5173"})(next)

	req := httptest.NewRequest(http.MethodOptions, "/login", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want unset for disallowed origin", got)
	}
	if !nextCalled {
		t.Error("next handler was not called, want request to pass through untouched")
	}
}

func TestCORS_SimpleRequestFromAllowedOrigin(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS([]string{"http://localhost:5173"})(next)

	req := httptest.NewRequest(http.MethodGet, "/transactions", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !nextCalled {
		t.Error("next handler was not called for a simple request")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:5173")
	}
	varyValues := rec.Header().Values("Vary")
	found := false
	for _, v := range varyValues {
		if v == "Origin" {
			found = true
		}
	}
	if !found {
		t.Errorf("Vary = %v, want it to contain Origin", varyValues)
	}
}

func TestCORS_NoOriginHeader(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS([]string{"http://localhost:5173"})(next)

	req := httptest.NewRequest(http.MethodGet, "/transactions", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !nextCalled {
		t.Error("next handler was not called for a request with no Origin header")
	}
	h := rec.Header()
	for _, name := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers", "Access-Control-Max-Age", "Vary"} {
		if got := h.Get(name); got != "" {
			t.Errorf("%s = %q, want unset when no Origin header is present", name, got)
		}
	}
}

func TestRouter_CORSPreflightAnswersBeforeRequireAuth(t *testing.T) {
	h := NewHealthHandler(fakePinger{})
	router := NewRouter(RouterDeps{
		Logger:             testLogger(),
		Health:             h,
		CORSAllowedOrigins: []string{"http://localhost:5173"},
		Modules: []RouteRegistrar{
			registrar(func(r chi.Router) {
				r.Post("/anything-protected", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
			}),
		},
		RequireAuth: func(http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized) // would reject the preflight if reached
			})
		},
	})

	req := httptest.NewRequest(http.MethodOptions, "/anything-protected", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (CORS must answer preflight before RequireAuth runs)", rec.Code)
	}
}
