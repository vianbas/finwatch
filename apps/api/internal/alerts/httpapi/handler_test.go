package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vianbas/finwatch/apps/api/internal/alerts"
	alerthttp "github.com/vianbas/finwatch/apps/api/internal/alerts/httpapi"
	"github.com/vianbas/finwatch/apps/api/internal/rules"
)

// stubRepo is a configurable test double for alerts.Repository.
type stubRepo struct {
	getResult        alerts.Alert
	getErr           error
	listResult       alerts.Page
	listErr          error
	transitionResult alerts.Alert
	transitionErr    error
}

func (s *stubRepo) Raise(_ context.Context, _ alerts.Raise) (alerts.Alert, bool, error) {
	return alerts.Alert{}, false, nil
}

func (s *stubRepo) Get(_ context.Context, _ string) (alerts.Alert, error) {
	return s.getResult, s.getErr
}

func (s *stubRepo) List(_ context.Context, _ alerts.PageQuery) (alerts.Page, error) {
	return s.listResult, s.listErr
}

func (s *stubRepo) Transition(_ context.Context, _ string, _, _ alerts.Status) (alerts.Alert, error) {
	return s.transitionResult, s.transitionErr
}

type stubRulesRepo struct{}

func (s *stubRulesRepo) ListEnabled(_ context.Context) ([]rules.Rule, error) { return nil, nil }
func (s *stubRulesRepo) EnsureDefaults(_ context.Context) error              { return nil }

func newHandler(repo alerts.Repository) http.Handler {
	svc := alerts.NewService(repo, &stubRulesRepo{}, slog.Default())
	r := chi.NewRouter()
	alerthttp.NewHandler(svc, slog.Default()).RegisterRoutes(r)
	return r
}

func fixedAlert(id string, status alerts.Status) alerts.Alert {
	return alerts.Alert{
		ID:            id,
		RuleID:        "rule-1",
		TransactionID: "txn-1",
		Status:        status,
		Severity:      "high",
		CreatedAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// decode is a test-local helper that deserialises JSON into a map.
func decode(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode body: %v\nbody: %s", err, body)
	}
	return m
}

func TestHandler_List_OK(t *testing.T) {
	al := fixedAlert("a1", alerts.StatusOpen)
	repo := &stubRepo{listResult: alerts.Page{Items: []alerts.Alert{al}}}
	h := newHandler(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	body := decode(t, rec.Body.Bytes())
	items, ok := body["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("want 1 item in response, got %v", body)
	}
}

func TestHandler_List_NoCursor_WhenNoNextPage(t *testing.T) {
	repo := &stubRepo{listResult: alerts.Page{Items: []alerts.Alert{fixedAlert("a1", alerts.StatusOpen)}}}
	h := newHandler(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	h.ServeHTTP(rec, req)

	body := decode(t, rec.Body.Bytes())
	if body["nextCursor"] != nil {
		t.Errorf("nextCursor=%v, want null", body["nextCursor"])
	}
}

func TestHandler_List_InvalidLimit_Returns400(t *testing.T) {
	h := newHandler(&stubRepo{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alerts?limit=abc", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestHandler_List_LimitOutOfRange_Returns400(t *testing.T) {
	h := newHandler(&stubRepo{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/alerts?limit=%d", alerts.MaxPageLimit+1), nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestHandler_List_InvalidStatus_Returns400(t *testing.T) {
	h := newHandler(&stubRepo{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alerts?status=unknown", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestHandler_List_InvalidCursor_Returns400(t *testing.T) {
	h := newHandler(&stubRepo{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alerts?cursor=!!!notbase64", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestHandler_Get_OK(t *testing.T) {
	al := fixedAlert("a1", alerts.StatusOpen)
	repo := &stubRepo{getResult: al}
	h := newHandler(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alerts/a1", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	body := decode(t, rec.Body.Bytes())
	if body["id"] != "a1" {
		t.Errorf("id=%v, want a1", body["id"])
	}
	if body["status"] != "open" {
		t.Errorf("status=%v, want open", body["status"])
	}
}

func TestHandler_Get_NotFound_Returns404(t *testing.T) {
	repo := &stubRepo{getErr: alerts.ErrNotFound}
	h := newHandler(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alerts/no-such-id", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
	body := decode(t, rec.Body.Bytes())
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("error.code=%v, want NOT_FOUND", errObj["code"])
	}
}

func TestHandler_Acknowledge_OK(t *testing.T) {
	open := fixedAlert("a1", alerts.StatusOpen)
	acked := fixedAlert("a1", alerts.StatusAcknowledged)
	repo := &stubRepo{getResult: open, transitionResult: acked}
	h := newHandler(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alerts/a1/acknowledge", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	body := decode(t, rec.Body.Bytes())
	if body["status"] != "acknowledged" {
		t.Errorf("status=%v, want acknowledged", body["status"])
	}
}

func TestHandler_Acknowledge_NotFound_Returns404(t *testing.T) {
	repo := &stubRepo{getErr: alerts.ErrNotFound}
	h := newHandler(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alerts/no-such-id/acknowledge", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
}

func TestHandler_Acknowledge_InvalidTransition_Returns409(t *testing.T) {
	// Alert is already resolved; open→acknowledged is invalid.
	repo := &stubRepo{getResult: fixedAlert("a1", alerts.StatusResolved)}
	h := newHandler(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alerts/a1/acknowledge", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d, want 409", rec.Code)
	}
	body := decode(t, rec.Body.Bytes())
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "INVALID_TRANSITION" {
		t.Errorf("error.code=%v, want INVALID_TRANSITION", errObj["code"])
	}
}

func TestHandler_Resolve_OK(t *testing.T) {
	acked := fixedAlert("a1", alerts.StatusAcknowledged)
	resolved := fixedAlert("a1", alerts.StatusResolved)
	repo := &stubRepo{getResult: acked, transitionResult: resolved}
	h := newHandler(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alerts/a1/resolve", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	body := decode(t, rec.Body.Bytes())
	if body["status"] != "resolved" {
		t.Errorf("status=%v, want resolved", body["status"])
	}
}

func TestHandler_Resolve_InvalidTransition_Returns409(t *testing.T) {
	// Alert is open; acknowledged→resolved requires acknowledged state first.
	repo := &stubRepo{getResult: fixedAlert("a1", alerts.StatusOpen)}
	h := newHandler(repo)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alerts/a1/resolve", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d, want 409", rec.Code)
	}
}
