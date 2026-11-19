package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
	"github.com/vianbas/finwatch/apps/api/internal/platform/httpserver"
	"github.com/vianbas/finwatch/apps/api/internal/rules"
)

const (
	testSecret = "test-only-signing-secret-at-least-32-chars"
	ruleID     = "11111111-1111-1111-1111-111111111111"
)

type fakeStore struct {
	rules    []rules.Rule
	setCalls int
	lastID   string
	lastOn   bool
	setErr   error
	listErr  error
}

func (f *fakeStore) List(context.Context) ([]rules.Rule, error) {
	return f.rules, f.listErr
}

func (f *fakeStore) SetEnabled(_ context.Context, id string, enabled bool) (rules.Rule, error) {
	f.setCalls++
	f.lastID, f.lastOn = id, enabled
	if f.setErr != nil {
		return rules.Rule{}, f.setErr
	}
	r := f.rules[0]
	r.Enabled = enabled
	return r, nil
}

func newFakeStore() *fakeStore {
	return &fakeStore{rules: []rules.Rule{{
		ID: ruleID, Name: "high-value-debit", Field: rules.FieldAmountMinor, Operator: rules.OpGTE,
		Value: "500000", Severity: "high", Enabled: true,
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}}}
}

// newRouter mounts the handler exactly as cmd/api does: as a protected module
// behind the real RequireAuth middleware.
func newRouter(t *testing.T, store Store) (http.Handler, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	return httpserver.NewRouter(httpserver.RouterDeps{
		Logger:      log,
		Health:      httpserver.NewHealthHandler(nil),
		Modules:     []httpserver.RouteRegistrar{NewHandler(store, log)},
		RequireAuth: auth.RequireAuth(auth.NewVerifier([]byte(testSecret))),
	}), &logs
}

func token(t *testing.T, secret string, role auth.Role) string {
	t.Helper()
	tok, err := auth.NewIssuer([]byte(secret), time.Minute).Issue("user-"+string(role), string(role)+"@example.com", role)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return tok
}

func do(t *testing.T, h http.Handler, method, path, bearer, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Error struct{ Code string } `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope %q: %v", rec.Body.String(), err)
	}
	return env.Error.Code
}

func TestList_AnyAuthenticatedRole(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleOperator, auth.RoleAdmin} {
		t.Run(string(role), func(t *testing.T) {
			h, _ := newRouter(t, newFakeStore())
			rec := do(t, h, http.MethodGet, "/rules", token(t, testSecret, role), "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
			}
			want := `{"items":[{"id":"` + ruleID + `","name":"high-value-debit","field":"amount_minor",` +
				`"operator":"gte","value":"500000","severity":"high","enabled":true,"createdAt":"2026-01-02T03:04:05Z"}]}`
			if got := strings.TrimSpace(rec.Body.String()); got != want {
				t.Errorf("body =\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func TestUpdate_RoleEnforcement(t *testing.T) {
	tests := []struct {
		name       string
		bearer     func(t *testing.T) string
		wantStatus int
		wantCode   string
		wantCalls  int
	}{
		{"admin may update", func(t *testing.T) string { return token(t, testSecret, auth.RoleAdmin) }, http.StatusOK, "", 1},
		{"operator is forbidden", func(t *testing.T) string { return token(t, testSecret, auth.RoleOperator) }, http.StatusForbidden, "FORBIDDEN", 0},
		{"no token", func(*testing.T) string { return "" }, http.StatusUnauthorized, "UNAUTHORIZED", 0},
		{"admin claim with forged signature", func(t *testing.T) string {
			return token(t, "another-secret-that-is-also-32-chars-long", auth.RoleAdmin)
		}, http.StatusUnauthorized, "UNAUTHORIZED", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			h, _ := newRouter(t, store)
			rec := do(t, h, http.MethodPatch, "/rules/"+ruleID, tt.bearer(t), `{"enabled":false}`)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantCode != "" && errorCode(t, rec) != tt.wantCode {
				t.Errorf("code = %q, want %q", errorCode(t, rec), tt.wantCode)
			}
			if store.setCalls != tt.wantCalls {
				t.Errorf("SetEnabled called %d times, want %d", store.setCalls, tt.wantCalls)
			}
		})
	}
}

func TestUpdate_AdminSuccess(t *testing.T) {
	store := newFakeStore()
	h, logs := newRouter(t, store)

	rec := do(t, h, http.MethodPatch, "/rules/"+ruleID, token(t, testSecret, auth.RoleAdmin), `{"enabled": false}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if store.lastID != ruleID || store.lastOn {
		t.Errorf("SetEnabled(%q, %v), want (%q, false)", store.lastID, store.lastOn, ruleID)
	}
	var got ruleDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Enabled || got.ID != ruleID {
		t.Errorf("response = %+v (err %v), want rule %s disabled", got, err, ruleID)
	}
	line := logs.String()
	for _, want := range []string{`"msg":"rule updated"`, `"rule_id":"` + ruleID + `"`, `"enabled":false`, `"user_id":"user-admin"`} {
		if !strings.Contains(line, want) {
			t.Errorf("log missing %s:\n%s", want, line)
		}
	}
	if strings.Contains(line, "@example.com") || strings.Contains(line, "Bearer") {
		t.Errorf("log leaks email or token:\n%s", line)
	}
}

func TestUpdate_RejectsInvalidBodies(t *testing.T) {
	bodies := map[string]string{
		"empty":         ``,
		"malformed":     `{"enabled":`,
		"missing field": `{}`,
		"null":          `{"enabled":null}`,
		"not a bool":    `{"enabled":"false"}`,
		"unknown field": `{"enabled":true,"severity":"low"}`,
		"trailing data": `{"enabled":true}{"enabled":false}`,
		"oversized":     `{"enabled":true,"pad":"` + strings.Repeat("x", maxUpdateBodyBytes) + `"}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			store := newFakeStore()
			h, _ := newRouter(t, store)
			rec := do(t, h, http.MethodPatch, "/rules/"+ruleID, token(t, testSecret, auth.RoleAdmin), body)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "INVALID_BODY" {
				t.Fatalf("status = %d body %s, want 400 INVALID_BODY", rec.Code, rec.Body)
			}
			if store.setCalls != 0 {
				t.Errorf("SetEnabled called on invalid body")
			}
		})
	}
}

func TestUpdate_StoreErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not found", rules.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"database failure", errors.New("connection refused"), http.StatusInternalServerError, "INTERNAL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			store.setErr = tt.err
			h, _ := newRouter(t, store)
			rec := do(t, h, http.MethodPatch, "/rules/"+ruleID, token(t, testSecret, auth.RoleAdmin), `{"enabled":true}`)
			if rec.Code != tt.wantStatus || errorCode(t, rec) != tt.wantCode {
				t.Errorf("status = %d code %q, want %d %q", rec.Code, errorCode(t, rec), tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestList_StoreError(t *testing.T) {
	store := newFakeStore()
	store.listErr = errors.New("connection refused")
	h, _ := newRouter(t, store)
	rec := do(t, h, http.MethodGet, "/rules", token(t, testSecret, auth.RoleOperator), "")
	if rec.Code != http.StatusInternalServerError || errorCode(t, rec) != "INTERNAL" {
		t.Errorf("status = %d body %s, want 500 INTERNAL", rec.Code, rec.Body)
	}
}
