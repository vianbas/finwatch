// Package httpapi exposes the auth module over HTTP: login and the current
// user. It validates input at the boundary and holds no business logic.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
	"github.com/vianbas/finwatch/apps/api/internal/platform/web"
)

// maxLoginBodyBytes bounds the size of the POST /login request body. /login
// is public and unauthenticated, so an unbounded body would let an anonymous
// caller exhaust server memory; 8 KiB comfortably covers any real
// email/password payload.
const maxLoginBodyBytes = 8 << 10 // 8 KiB

// Handler serves the auth HTTP endpoints.
type Handler struct {
	svc *auth.Service
	log *slog.Logger
}

// NewHandler constructs a Handler.
func NewHandler(svc *auth.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// RegisterPublicRoutes mounts routes that do not require authentication.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/login", h.login)
}

// RegisterProtectedRoutes mounts routes that require a valid bearer token.
// The caller is responsible for applying auth.RequireAuth to this router.
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Get("/me", h.me)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type loginResponse struct {
	AccessToken string  `json:"accessToken"`
	User        userDTO `json:"user"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		web.WriteError(w, http.StatusBadRequest, "INVALID_BODY", "request body must be valid JSON")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		web.WriteError(w, http.StatusBadRequest, "INVALID_BODY", "email and password are required")
		return
	}

	token, user, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			web.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email or password is incorrect")
			return
		}
		h.log.ErrorContext(r.Context(), "login failed", slog.String("error", err.Error()))
		web.WriteError(w, http.StatusInternalServerError, "INTERNAL", "unexpected error")
		return
	}
	web.WriteJSON(w, http.StatusOK, loginResponse{AccessToken: token, User: toUserDTO(user)})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		web.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
		return
	}
	web.WriteJSON(w, http.StatusOK, userDTO{ID: claims.UserID, Email: claims.Email, Role: string(claims.Role)})
}

func toUserDTO(u auth.User) userDTO {
	return userDTO{ID: u.ID, Email: u.Email, Role: string(u.Role)}
}
