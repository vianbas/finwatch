package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
	"github.com/vianbas/finwatch/apps/api/internal/auth/httpapi"
)

const handlerTestSecret = "test-signing-secret-must-be-at-least-32-bytes"

type fakeUserStore struct {
	usersByEmail map[string]auth.User
}

func (f *fakeUserStore) GetUserByEmail(_ context.Context, email string) (auth.User, error) {
	u, ok := f.usersByEmail[email]
	if !ok {
		return auth.User{}, auth.ErrUserNotFound
	}
	return u, nil
}

func newRouter(t *testing.T, users ...auth.User) http.Handler {
	t.Helper()
	byEmail := make(map[string]auth.User, len(users))
	for _, u := range users {
		byEmail[u.Email] = u
	}
	issuer := auth.NewIssuer([]byte(handlerTestSecret), 15*time.Minute)
	verifier := auth.NewVerifier([]byte(handlerTestSecret))
	svc := auth.NewService(&fakeUserStore{usersByEmail: byEmail}, issuer)
	h := httpapi.NewHandler(svc, slog.Default())

	r := chi.NewRouter()
	h.RegisterPublicRoutes(r)
	r.Group(func(pr chi.Router) {
		pr.Use(auth.RequireAuth(verifier))
		h.RegisterProtectedRoutes(pr)
	})
	return r
}

func userWithPassword(t *testing.T, email, password string, role auth.Role) auth.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	return auth.User{ID: "user-1", Email: email, PasswordHash: hash, Role: role}
}

func decode(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode body: %v\nbody: %s", err, body)
	}
	return m
}

func TestLogin_ValidCredentials(t *testing.T) {
	user := userWithPassword(t, "operator@example.com", "correct-password", auth.RoleOperator)
	router := newRouter(t, user)

	body, _ := json.Marshal(map[string]string{"email": "operator@example.com", "password": "correct-password"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	resp := decode(t, rec.Body.Bytes())
	if resp["accessToken"] == "" || resp["accessToken"] == nil {
		t.Errorf("want non-empty accessToken, got %v", resp["accessToken"])
	}
	user_, ok := resp["user"].(map[string]any)
	if !ok || user_["email"] != "operator@example.com" || user_["role"] != "operator" {
		t.Errorf("user field = %v, want email/role operator@example.com/operator", resp["user"])
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	user := userWithPassword(t, "operator@example.com", "correct-password", auth.RoleOperator)
	router := newRouter(t, user)

	body, _ := json.Marshal(map[string]string{"email": "operator@example.com", "password": "wrong-password"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	router := newRouter(t)

	body, _ := json.Marshal(map[string]string{"email": "", "password": ""})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestLogin_BodyTooLarge(t *testing.T) {
	router := newRouter(t)

	// Build a >8 KiB JSON body: a valid-shaped payload padded out with a
	// filler field so it still decodes as JSON up to the point the reader
	// cuts it off.
	padding := bytes.Repeat([]byte("a"), 9<<10)
	body, _ := json.Marshal(map[string]string{
		"email":    "operator@example.com",
		"password": "correct-password",
		"padding":  string(padding),
	})
	if len(body) <= 8<<10 {
		t.Fatalf("test body must exceed 8 KiB, got %d bytes", len(body))
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
	resp := decode(t, rec.Body.Bytes())
	errObj, ok := resp["error"].(map[string]any)
	if !ok || errObj["code"] != "INVALID_BODY" {
		t.Errorf("error = %v, want code INVALID_BODY", resp["error"])
	}
}

func TestMe_ValidToken_ReturnsClaims(t *testing.T) {
	user := userWithPassword(t, "operator@example.com", "correct-password", auth.RoleOperator)
	router := newRouter(t, user)

	loginBody, _ := json.Marshal(map[string]string{"email": "operator@example.com", "password": "correct-password"})
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginBody)))
	token := decode(t, loginRec.Body.Bytes())["accessToken"].(string)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	resp := decode(t, rec.Body.Bytes())
	if resp["email"] != "operator@example.com" || resp["role"] != "operator" {
		t.Errorf("got %v, want email/role operator@example.com/operator", resp)
	}
}

func TestMe_NoToken_Returns401(t *testing.T) {
	router := newRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
