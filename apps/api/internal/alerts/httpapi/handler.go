// Package httpapi exposes the alerts module over HTTP: listing, retrieval, and
// lifecycle transitions. It validates input at the boundary and renders the
// shared canonical envelope; it holds no business logic.
package httpapi

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vianbas/finwatch/apps/api/internal/alerts"
	"github.com/vianbas/finwatch/apps/api/internal/platform/web"
)

// Handler serves the alerts HTTP endpoints.
type Handler struct {
	svc *alerts.Service
	log *slog.Logger
}

// NewHandler constructs a Handler.
func NewHandler(svc *alerts.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// RegisterRoutes mounts the alerts routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/alerts", h.list)
	r.Get("/alerts/{id}", h.get)
	r.Post("/alerts/{id}/acknowledge", h.acknowledge)
	r.Post("/alerts/{id}/resolve", h.resolve)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q, err := parsePageQuery(r)
	if err != nil {
		web.WriteError(w, http.StatusBadRequest, "INVALID_QUERY", err.Error())
		return
	}
	page, err := h.svc.List(r.Context(), q)
	if err != nil {
		h.log.ErrorContext(r.Context(), "list alerts failed", slog.String("error", err.Error()))
		web.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to list alerts")
		return
	}
	web.WriteJSON(w, http.StatusOK, toPageResponse(page))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	alert, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, toDTO(alert))
}

func (h *Handler) acknowledge(w http.ResponseWriter, r *http.Request) {
	alert, err := h.svc.Acknowledge(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, toDTO(alert))
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	alert, err := h.svc.Resolve(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, toDTO(alert))
}

// writeServiceError maps domain errors to HTTP responses.
func (h *Handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, alerts.ErrNotFound):
		web.WriteError(w, http.StatusNotFound, "NOT_FOUND", "alert not found")
	case errors.Is(err, alerts.ErrInvalidTransition):
		web.WriteError(w, http.StatusConflict, "INVALID_TRANSITION", "alert is not in a state that allows this transition")
	default:
		h.log.ErrorContext(r.Context(), "alert request failed", slog.String("error", err.Error()))
		web.WriteError(w, http.StatusInternalServerError, "INTERNAL", "unexpected error")
	}
}

type alertDTO struct {
	ID            string `json:"id"`
	RuleID        string `json:"ruleId"`
	TransactionID string `json:"transactionId"`
	Status        string `json:"status"`
	Severity      string `json:"severity"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type pageResponse struct {
	Items      []alertDTO `json:"items"`
	NextCursor *string    `json:"nextCursor"`
}

func toDTO(a alerts.Alert) alertDTO {
	return alertDTO{
		ID:            a.ID,
		RuleID:        a.RuleID,
		TransactionID: a.TransactionID,
		Status:        string(a.Status),
		Severity:      a.Severity,
		CreatedAt:     a.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:     a.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func toPageResponse(p alerts.Page) pageResponse {
	items := make([]alertDTO, 0, len(p.Items))
	for _, a := range p.Items {
		items = append(items, toDTO(a))
	}
	resp := pageResponse{Items: items}
	if p.Next != nil {
		c := encodeCursor(*p.Next)
		resp.NextCursor = &c
	}
	return resp
}

var validStatuses = map[string]alerts.Status{
	"open":         alerts.StatusOpen,
	"acknowledged": alerts.StatusAcknowledged,
	"resolved":     alerts.StatusResolved,
}

func parsePageQuery(r *http.Request) (alerts.PageQuery, error) {
	q := alerts.PageQuery{}

	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return q, fmt.Errorf("limit must be an integer")
		}
		if n < 1 || n > alerts.MaxPageLimit {
			return q, fmt.Errorf("limit must be between 1 and %d", alerts.MaxPageLimit)
		}
		q.Limit = n
	}

	if raw := r.URL.Query().Get("status"); raw != "" {
		st, ok := validStatuses[raw]
		if !ok {
			return q, fmt.Errorf("status must be one of open, acknowledged, resolved")
		}
		q.Status = st
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

func encodeCursor(c alerts.Cursor) string {
	raw := fmt.Sprintf("%s|%s", c.CreatedAt.UTC().Format(time.RFC3339Nano), c.ID)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(token string) (alerts.Cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return alerts.Cursor{}, err
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return alerts.Cursor{}, fmt.Errorf("malformed cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return alerts.Cursor{}, err
	}
	if parts[1] == "" {
		return alerts.Cursor{}, fmt.Errorf("malformed cursor id")
	}
	return alerts.Cursor{CreatedAt: ts.UTC(), ID: parts[1]}, nil
}
