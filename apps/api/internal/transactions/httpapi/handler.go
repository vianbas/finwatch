// Package httpapi exposes the transactions module over HTTP. It validates input
// at the trust boundary, delegates to the domain service, and renders responses
// with the shared canonical envelope. It contains no business logic.
package httpapi

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vianbas/finwatch/apps/api/internal/platform/web"
	"github.com/vianbas/finwatch/apps/api/internal/transactions"
)

// Handler serves the transactions HTTP endpoints.
type Handler struct {
	svc *transactions.Service
	log *slog.Logger
}

// NewHandler constructs a Handler.
func NewHandler(svc *transactions.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// RegisterRoutes mounts the module's routes on the application router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/transactions", h.list)
}

// list handles GET /transactions with keyset pagination via opaque cursor.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q, err := parsePageQuery(r)
	if err != nil {
		web.WriteError(w, http.StatusBadRequest, "INVALID_QUERY", err.Error())
		return
	}

	page, err := h.svc.List(r.Context(), q)
	if err != nil {
		h.log.ErrorContext(r.Context(), "list transactions failed", slog.String("error", err.Error()))
		web.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to list transactions")
		return
	}

	web.WriteJSON(w, http.StatusOK, toPageResponse(page))
}

// transactionDTO is the wire representation. Money is integer minor units;
// occurredAt is RFC 3339.
type transactionDTO struct {
	ID          string `json:"id"`
	AccountID   string `json:"accountId"`
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
	Direction   string `json:"direction"`
	Status      string `json:"status"`
	OccurredAt  string `json:"occurredAt"`
}

type pageResponse struct {
	Items      []transactionDTO `json:"items"`
	NextCursor *string          `json:"nextCursor"`
}

func toPageResponse(p transactions.Page) pageResponse {
	items := make([]transactionDTO, 0, len(p.Items))
	for _, t := range p.Items {
		items = append(items, transactionDTO{
			ID:          t.ID,
			AccountID:   t.AccountID,
			AmountMinor: t.AmountMinor,
			Currency:    t.Currency,
			Direction:   string(t.Direction),
			Status:      string(t.Status),
			OccurredAt:  t.OccurredAt.UTC().Format(time.RFC3339Nano),
		})
	}
	resp := pageResponse{Items: items}
	if p.Next != nil {
		c := encodeCursor(*p.Next)
		resp.NextCursor = &c
	}
	return resp
}

// parsePageQuery validates the limit and cursor query parameters.
func parsePageQuery(r *http.Request) (transactions.PageQuery, error) {
	q := transactions.PageQuery{}

	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return q, fmt.Errorf("limit must be an integer")
		}
		if n < 1 || n > transactions.MaxPageLimit {
			return q, fmt.Errorf("limit must be between 1 and %d", transactions.MaxPageLimit)
		}
		q.Limit = n
	}

	if raw := r.URL.Query().Get("cursor"); raw != "" {
		cur, err := decodeCursor(raw)
		if err != nil {
			return q, fmt.Errorf("cursor is invalid")
		}
		q.After = &cur
	}

	return q, nil
}

// encodeCursor renders a cursor as an opaque, URL-safe token.
func encodeCursor(c transactions.Cursor) string {
	raw := fmt.Sprintf("%s|%s", c.OccurredAt.UTC().Format(time.RFC3339Nano), c.ID)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor parses an opaque cursor token.
func decodeCursor(token string) (transactions.Cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return transactions.Cursor{}, err
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return transactions.Cursor{}, fmt.Errorf("malformed cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return transactions.Cursor{}, err
	}
	if parts[1] == "" {
		return transactions.Cursor{}, fmt.Errorf("malformed cursor id")
	}
	return transactions.Cursor{OccurredAt: ts.UTC(), ID: parts[1]}, nil
}
