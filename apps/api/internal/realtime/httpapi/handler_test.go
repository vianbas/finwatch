package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
	"github.com/vianbas/finwatch/apps/api/internal/platform/httpserver"
	"github.com/vianbas/finwatch/apps/api/internal/realtime"
)

const serverTimeout = 100 * time.Millisecond

// fakeVerifier accepts "valid" (long-lived) and "short" (expires soon).
type fakeVerifier struct{}

func (fakeVerifier) Verify(token string) (auth.Claims, error) {
	switch token {
	case "valid":
		return auth.Claims{UserID: "u1", Role: auth.RoleOperator, ExpiresAt: time.Now().Add(time.Hour)}, nil
	case "short":
		return auth.Claims{UserID: "u1", Role: auth.RoleOperator, ExpiresAt: time.Now().Add(150 * time.Millisecond)}, nil
	default:
		return auth.Claims{}, errors.New("invalid token")
	}
}

// newServer serves the handler behind AccessLog (exercising Unwrap) on a
// server with deliberately short read/write timeouts, which the handler must
// clear for long-lived streams.
func newServer(t *testing.T) (*httptest.Server, *realtime.Hub) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := realtime.NewHub(log)
	h := NewHandler(hub, fakeVerifier{}, []string{"http://allowed.example"}, log)

	r := chi.NewRouter()
	r.Use(httpserver.AccessLog(log))
	h.RegisterRoutes(r)

	srv := httptest.NewUnstartedServer(r)
	srv.Config.ReadTimeout = serverTimeout
	srv.Config.WriteTimeout = serverTimeout
	srv.Start()
	t.Cleanup(srv.Close)
	return srv, hub
}

func wsURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
}

func dial(t *testing.T, srv *httptest.Server, protocols ...string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(ctx, wsURL(srv), &websocket.DialOptions{Subprotocols: protocols})
	if conn != nil {
		t.Cleanup(func() { _ = conn.CloseNow() })
	}
	return conn, resp, err
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func readEnvelope(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	typ, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("message type = %v, want text", typ)
	}
	var env map[string]any
	if err := json.Unmarshal(msg, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return env
}

func readCloseStatus(t *testing.T, conn *websocket.Conn) websocket.StatusCode {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := conn.Read(ctx)
	return websocket.CloseStatus(err)
}

func event(id int64) realtime.Event {
	return realtime.Event{
		ID:         id,
		Type:       "transaction.observed",
		Payload:    json.RawMessage(`{"amountMinor":1250}`),
		OccurredAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func TestServe_RejectsHandshake(t *testing.T) {
	srv, hub := newServer(t)
	tests := []struct {
		name      string
		protocols []string
		want      int
	}{
		{"no subprotocols", nil, http.StatusBadRequest},
		{"token without finwatch.v1", []string{"bearer.valid"}, http.StatusBadRequest},
		{"missing token", []string{"finwatch.v1"}, http.StatusUnauthorized},
		{"invalid token", []string{"finwatch.v1", "bearer.forged"}, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, resp, err := dial(t, srv, tt.protocols...)
			if err == nil {
				t.Fatal("dial succeeded, want rejection")
			}
			if resp == nil || resp.StatusCode != tt.want {
				t.Fatalf("status = %v, want %d", resp, tt.want)
			}
		})
	}
	if hub.Len() != 0 {
		t.Errorf("hub has %d subscribers after rejected handshakes", hub.Len())
	}
}

func TestServe_RejectsDisallowedOrigin(t *testing.T) {
	srv, _ := newServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, wsURL(srv), &websocket.DialOptions{
		Subprotocols: []string{"finwatch.v1", "bearer.valid"},
		HTTPHeader:   http.Header{"Origin": []string{"http://evil.example"}},
	})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("err=%v resp=%v, want 403", err, resp)
	}
}

func TestServe_StreamsEventsBeyondServerTimeouts(t *testing.T) {
	srv, hub := newServer(t)
	conn, _, err := dial(t, srv, "finwatch.v1", "bearer.valid")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if conn.Subprotocol() != "finwatch.v1" {
		t.Errorf("subprotocol = %q, want finwatch.v1", conn.Subprotocol())
	}
	waitFor(t, func() bool { return hub.Len() == 1 })

	// Outlive the server's read/write timeouts before the first event.
	time.Sleep(3 * serverTimeout)
	hub.Publish(event(7))
	hub.Publish(event(7)) // a relay redelivery

	for range 2 {
		env := readEnvelope(t, conn)
		if env["id"] != "7" || env["type"] != "transaction.observed" || env["version"] != float64(1) {
			t.Errorf("envelope = %v", env)
		}
		if env["occurredAt"] != "2026-01-02T03:04:05Z" {
			t.Errorf("occurredAt = %v", env["occurredAt"])
		}
	}
}

func TestServe_AllowsListedOrigin(t *testing.T) {
	srv, _ := newServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL(srv), &websocket.DialOptions{
		Subprotocols: []string{"finwatch.v1", "bearer.valid"},
		HTTPHeader:   http.Header{"Origin": []string{"http://allowed.example"}},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.CloseNow()
}

func TestServe_ClientDisconnectUnsubscribes(t *testing.T) {
	srv, hub := newServer(t)
	conn, _, err := dial(t, srv, "finwatch.v1", "bearer.valid")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	waitFor(t, func() bool { return hub.Len() == 1 })

	_ = conn.Close(websocket.StatusNormalClosure, "bye")

	waitFor(t, func() bool { return hub.Len() == 0 })
}

func TestServe_HubShutdownClosesWithGoingAway(t *testing.T) {
	srv, hub := newServer(t)
	conn, _, err := dial(t, srv, "finwatch.v1", "bearer.valid")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	waitFor(t, func() bool { return hub.Len() == 1 })

	hub.Close()

	if got := readCloseStatus(t, conn); got != websocket.StatusGoingAway {
		t.Errorf("close status = %v, want %v", got, websocket.StatusGoingAway)
	}
}

func TestServe_TokenExpiryClosesWithPolicyViolation(t *testing.T) {
	srv, _ := newServer(t)
	conn, _, err := dial(t, srv, "finwatch.v1", "bearer.short")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if got := readCloseStatus(t, conn); got != websocket.StatusPolicyViolation {
		t.Errorf("close status = %v, want %v", got, websocket.StatusPolicyViolation)
	}
}

func TestServe_SlowClientClosesWithTryAgainLater(t *testing.T) {
	srv, hub := newServer(t)
	conn, _, err := dial(t, srv, "finwatch.v1", "bearer.valid")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	waitFor(t, func() bool { return hub.Len() == 1 })

	// The client does not read; flood until the hub drops it.
	for i := int64(0); hub.Len() == 1; i++ {
		hub.Publish(event(i))
		if i > 100_000 {
			t.Fatal("hub never dropped the non-reading client")
		}
	}

	// Drain buffered events until the close frame arrives.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			if got := websocket.CloseStatus(err); got != websocket.StatusTryAgainLater {
				t.Errorf("close status = %v (err %v), want %v", got, err, websocket.StatusTryAgainLater)
			}
			return
		}
	}
}

func TestParseSubprotocols(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.Header.Add("Sec-WebSocket-Protocol", "finwatch.v1, bearer.a.b-c_d")
	token, ok := parseSubprotocols(r)
	if !ok || token != "a.b-c_d" {
		t.Errorf("got (%q, %v), want (a.b-c_d, true)", token, ok)
	}
}
