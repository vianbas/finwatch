package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
)

const mwTestSecret = "test-signing-secret-must-be-at-least-32-bytes"

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if ok {
			w.Header().Set("X-Role", string(claims.Role))
		}
		w.WriteHeader(http.StatusOK)
	})
}

func issueToken(t *testing.T, ttl time.Duration, role auth.Role) string {
	t.Helper()
	issuer := auth.NewIssuer([]byte(mwTestSecret), ttl)
	token, err := issuer.Issue("user-1", "operator@example.com", role)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return token
}

func TestRequireAuth_MissingToken(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_MalformedToken(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(okHandler())
	token := issueToken(t, -1*time.Minute, auth.RoleOperator)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_LowercaseBearerScheme(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(okHandler())
	token := issueToken(t, 15*time.Minute, auth.RoleOperator)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestRequireAuth_MissingTokenSetsWWWAuthenticate(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want %q", got, "Bearer")
	}
}

func TestRequireAuth_ValidToken(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(okHandler())
	token := issueToken(t, 15*time.Minute, auth.RoleOperator)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Role"); got != "operator" {
		t.Errorf("X-Role = %q, want operator", got)
	}
}

func TestRequireRole_OperatorDeniedFromAdminRoute(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(auth.RequireRole(auth.RoleAdmin)(okHandler()))
	token := issueToken(t, 15*time.Minute, auth.RoleOperator)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestRequireRole_AdminAllowed(t *testing.T) {
	verifier := auth.NewVerifier([]byte(mwTestSecret))
	handler := auth.RequireAuth(verifier)(auth.RequireRole(auth.RoleAdmin)(okHandler()))
	token := issueToken(t, 15*time.Minute, auth.RoleAdmin)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
