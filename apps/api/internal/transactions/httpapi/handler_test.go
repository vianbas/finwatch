package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vianbas/finwatch/apps/api/internal/transactions"
)

// stubRepo returns a fixed page and is enough to exercise the handler.
type stubRepo struct{ page transactions.Page }

func (s stubRepo) Save(context.Context, transactions.Transaction) (transactions.Transaction, error) {
	return transactions.Transaction{}, nil
}
func (s stubRepo) List(context.Context, transactions.PageQuery) (transactions.Page, error) {
	return s.page, nil
}

func newRouter(page transactions.Page) http.Handler {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	svc := transactions.NewService(stubRepo{page: page}, transactions.NewGenerator(1), logger)
	h := NewHandler(svc, logger)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func TestList_OK(t *testing.T) {
	occurred := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	page := transactions.Page{
		Items: []transactions.Transaction{{
			ID: "abc", AccountID: "ACC-1", AmountMinor: 2500, Currency: "USD",
			Direction: transactions.DirectionCredit, Status: transactions.StatusPosted,
			OccurredAt: occurred,
		}},
		Next: &transactions.Cursor{OccurredAt: occurred, ID: "abc"},
	}

	rec := httptest.NewRecorder()
	newRouter(page).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/transactions", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp pageResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(resp.Items))
	}
	got := resp.Items[0]
	if got.AmountMinor != 2500 || got.Direction != "credit" || got.AccountID != "ACC-1" {
		t.Errorf("unexpected item: %+v", got)
	}
	if got.OccurredAt != occurred.Format(time.RFC3339Nano) {
		t.Errorf("occurredAt = %q, want RFC3339 %q", got.OccurredAt, occurred.Format(time.RFC3339Nano))
	}
	if resp.NextCursor == nil {
		t.Fatal("expected nextCursor")
	}
	cur, err := decodeCursor(*resp.NextCursor)
	if err != nil {
		t.Fatalf("nextCursor not decodable: %v", err)
	}
	if cur.ID != "abc" || !cur.OccurredAt.Equal(occurred) {
		t.Errorf("decoded cursor = %+v", cur)
	}
}

func TestList_NoNextCursor(t *testing.T) {
	page := transactions.Page{Items: []transactions.Transaction{}, Next: nil}
	rec := httptest.NewRecorder()
	newRouter(page).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/transactions", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp pageResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.NextCursor != nil {
		t.Errorf("nextCursor = %v, want null", *resp.NextCursor)
	}
	if resp.Items == nil {
		t.Error("items should serialise as [] not null")
	}
}

func TestList_InvalidQuery(t *testing.T) {
	tests := []struct{ name, query string }{
		{"non-integer limit", "/transactions?limit=abc"},
		{"zero limit", "/transactions?limit=0"},
		{"over-max limit", "/transactions?limit=99999"},
		{"malformed cursor", "/transactions?cursor=not%20base64%21"},
	}
	page := transactions.Page{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newRouter(page).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.query, nil))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			var body map[string]map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body["error"]["code"] != "INVALID_QUERY" {
				t.Errorf("error code = %q, want INVALID_QUERY", body["error"]["code"])
			}
		})
	}
}
