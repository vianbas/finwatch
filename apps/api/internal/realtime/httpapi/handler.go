// Package httpapi exposes the realtime hub as the /ws WebSocket endpoint
// described in contracts/asyncapi.yaml. It authenticates the handshake,
// upgrades, and streams hub events to the client until either side ends it.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
	"github.com/vianbas/finwatch/apps/api/internal/platform/web"
	"github.com/vianbas/finwatch/apps/api/internal/realtime"
)

const (
	// subprotocol is the application protocol the server selects.
	subprotocol = "finwatch.v1"
	// bearerPrefix marks the subprotocol carrying the access token, since
	// browsers cannot set an Authorization header on a WebSocket handshake.
	bearerPrefix = "bearer."

	writeTimeout = 10 * time.Second
	pingInterval = 30 * time.Second
)

// TokenVerifier validates an access token. *auth.Verifier implements it.
type TokenVerifier interface {
	Verify(token string) (auth.Claims, error)
}

// Handler serves the /ws endpoint.
type Handler struct {
	hub            *realtime.Hub
	verifier       TokenVerifier
	originPatterns []string
	log            *slog.Logger
}

// NewHandler constructs a Handler. allowedOrigins are the browser origins
// (e.g. http://localhost:5173) permitted to open cross-origin connections;
// same-origin connections are always allowed.
func NewHandler(hub *realtime.Hub, verifier TokenVerifier, allowedOrigins []string, log *slog.Logger) *Handler {
	patterns := make([]string, 0, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			patterns = append(patterns, u.Host)
		}
	}
	return &Handler{hub: hub, verifier: verifier, originPatterns: patterns, log: log}
}

// RegisterRoutes mounts /ws. It authenticates the handshake itself, so it must
// be mounted outside the bearer-header auth middleware.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/ws", h.serve)
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	token, hasProtocol := parseSubprotocols(r)
	if !hasProtocol {
		web.WriteError(w, http.StatusBadRequest, "UNSUPPORTED_SUBPROTOCOL", "subprotocol "+subprotocol+" is required")
		return
	}
	if token == "" {
		w.Header().Set("WWW-Authenticate", "Bearer")
		web.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
		return
	}
	claims, err := h.verifier.Verify(token)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		web.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
		return
	}

	// The server's read/write timeouts are connection deadlines that survive
	// the upgrade; clear them so the stream outlives them. Pings and per-write
	// timeouts bound a dead peer instead.
	rc := http.NewResponseController(w)
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		h.log.ErrorContext(r.Context(), "realtime: clear read deadline failed", slog.String("error", err.Error()))
		web.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to open event stream")
		return
	}
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		h.log.ErrorContext(r.Context(), "realtime: clear write deadline failed", slog.String("error", err.Error()))
		web.WriteError(w, http.StatusInternalServerError, "INTERNAL", "failed to open event stream")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols:   []string{subprotocol},
		OriginPatterns: h.originPatterns,
	})
	if err != nil {
		// Accept has already written an error response (e.g. 403 for a
		// disallowed origin).
		h.log.InfoContext(r.Context(), "realtime: upgrade rejected", slog.String("error", err.Error()))
		return
	}
	defer func() { _ = conn.CloseNow() }()

	h.stream(r.Context(), conn, claims.ExpiresAt)
}

// stream relays hub events to conn until the client goes away, the hub drops
// the subscription, or the access token expires.
func (h *Handler) stream(ctx context.Context, conn *websocket.Conn, expiresAt time.Time) {
	sub := h.hub.Subscribe()
	defer sub.Close()

	// The stream is server-to-client only: CloseRead handles control frames
	// and closes with 1008 if the client sends a data message.
	ctx = conn.CloseRead(ctx)

	expiry := time.NewTimer(time.Until(expiresAt))
	defer expiry.Stop()
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.Done():
			switch sub.Reason() {
			case realtime.DropSlow:
				_ = conn.Close(websocket.StatusTryAgainLater, "client fell behind")
			case realtime.DropShutdown:
				_ = conn.Close(websocket.StatusGoingAway, "server shutting down")
			}
			return
		case <-expiry.C:
			_ = conn.Close(websocket.StatusPolicyViolation, "token expired")
			return
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				return
			}
		case msg := <-sub.Events():
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Write(wctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// parseSubprotocols reads the offered subprotocols, reporting the bearer
// token (empty if absent) and whether finwatch.v1 was offered.
func parseSubprotocols(r *http.Request) (token string, hasProtocol bool) {
	for _, header := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, p := range strings.Split(header, ",") {
			p = strings.TrimSpace(p)
			switch {
			case p == subprotocol:
				hasProtocol = true
			case strings.HasPrefix(p, bearerPrefix):
				token = strings.TrimPrefix(p, bearerPrefix)
			}
		}
	}
	return token, hasProtocol
}
