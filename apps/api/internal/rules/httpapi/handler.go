// Package httpapi exposes rule management over HTTP. Listing is open to any
// authenticated role; changing a rule requires the admin role, enforced here
// with auth.RequireRole so the check sits next to the route it protects.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
	"github.com/vianbas/finwatch/apps/api/internal/platform/web"
	"github.com/vianbas/finwatch/apps/api/internal/rules"
)

// maxUpdateBodyBytes bounds the PATCH body; a valid one is ~20 bytes.
const maxUpdateBodyBytes = 1 << 10

// Store is the rule persistence the handler needs.
type Store interface {
	List(ctx context.Context) ([]rules.Rule, error)
	SetEnabled(ctx context.Context, id string, enabled bool) (rules.Rule, error)
}

// Handler serves the rules HTTP endpoints.
type Handler struct {
	store Store
	log   *slog.Logger
}

// NewHandler constructs a Handler.
func NewHandler(store Store, log *slog.Logger) *Handler {
	return &Handler{store: store, log: log}
}

// RegisterRoutes mounts the rules routes. They must run behind
// auth.RequireAuth, which RequireRole relies on for the caller's claims.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/rules", h.list)
	r.With(auth.RequireRole(auth.RoleAdmin)).Patch("/rules/{id}", h.update)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	rs, err := h.store.List(r.Context())
	if err != nil {
		h.log.ErrorContext(r.Context(), "list rules failed", slog.String("error", err.Error()))
		web.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to list rules")
		return
	}
	items := make([]ruleDTO, len(rs))
	for i, rule := range rs {
		items[i] = toDTO(rule)
	}
	web.WriteJSON(w, http.StatusOK, ruleListDTO{Items: items})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	enabled, ok := decodeUpdate(w, r)
	if !ok {
		web.WriteError(w, http.StatusBadRequest, "INVALID_BODY", `body must be {"enabled": true|false}`)
		return
	}

	id := chi.URLParam(r, "id")
	rule, err := h.store.SetEnabled(r.Context(), id, enabled)
	if errors.Is(err, rules.ErrNotFound) {
		web.WriteError(w, http.StatusNotFound, "NOT_FOUND", "rule not found")
		return
	}
	if err != nil {
		h.log.ErrorContext(r.Context(), "update rule failed", slog.String("error", err.Error()))
		web.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to update rule")
		return
	}

	// Who changed which rule, for traceability. The user id is an opaque
	// synthetic identifier; emails and tokens are never logged.
	claims, _ := auth.ClaimsFromContext(r.Context())
	h.log.InfoContext(r.Context(), "rule updated",
		slog.String("rule_id", rule.ID),
		slog.Bool("enabled", rule.Enabled),
		slog.String("user_id", claims.UserID),
	)
	web.WriteJSON(w, http.StatusOK, toDTO(rule))
}

// decodeUpdate strictly parses {"enabled": bool}: bounded size, no unknown
// fields, the field required, and nothing after the object.
func decodeUpdate(w http.ResponseWriter, r *http.Request) (bool, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpdateBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := dec.Decode(&body); err != nil || body.Enabled == nil {
		return false, false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return false, false
	}
	return *body.Enabled, true
}

type ruleDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Field     string `json:"field"`
	Operator  string `json:"operator"`
	Value     string `json:"value"`
	Severity  string `json:"severity"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"createdAt"`
}

type ruleListDTO struct {
	Items []ruleDTO `json:"items"`
}

func toDTO(r rules.Rule) ruleDTO {
	return ruleDTO{
		ID:        r.ID,
		Name:      r.Name,
		Field:     string(r.Field),
		Operator:  string(r.Operator),
		Value:     r.Value,
		Severity:  r.Severity,
		Enabled:   r.Enabled,
		CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}
