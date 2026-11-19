package store

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests exercise real PostgreSQL behaviour. They run only when
// FINWATCH_TEST_DATABASE_URL points at a disposable database, e.g.:
//
//	make dev   # starts Postgres on localhost:5432
//	FINWATCH_TEST_DATABASE_URL='postgres://finwatch:finwatch_dev_password@localhost:5432/finwatch?sslmode=disable' \
//	  go test ./internal/realtime/store/...
//
// Without the env var they skip, so `make verify` stays green without a DB.

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("FINWATCH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set FINWATCH_TEST_DATABASE_URL to run store integration tests")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	resetSchema(t, pool)
	return pool
}

func resetSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	dir := filepath.Join("..", "..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var ups []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)
	for _, name := range ups {
		sql, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
}

func insertEvent(t *testing.T, pool *pgxpool.Pool, eventType, payload string, occurredAt time.Time) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO outbox_events (aggregate, event_type, payload, occurred_at)
		VALUES ('test', $1, $2, $3)
		RETURNING id
	`, eventType, payload, occurredAt).Scan(&id)
	if err != nil {
		t.Fatalf("insert outbox event: %v", err)
	}
	return id
}

func TestStore_ListUnpublishedOldestFirstAndMarkPublished(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	s := New(pool)

	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	later := insertEvent(t, pool, "alert.raised", `{"n":2}`, base.Add(time.Minute))
	earlier := insertEvent(t, pool, "transaction.observed", `{"n":1}`, base)

	events, err := s.ListUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("ListUnpublished: %v", err)
	}
	if len(events) != 2 || events[0].ID != earlier || events[1].ID != later {
		t.Fatalf("events = %+v, want ids [%d %d]", events, earlier, later)
	}
	e := events[0]
	if e.Type != "transaction.observed" || string(e.Payload) != `{"n": 1}` || !e.OccurredAt.Equal(base) {
		t.Errorf("event = {Type:%s Payload:%s OccurredAt:%s}", e.Type, e.Payload, e.OccurredAt)
	}
	if e.OccurredAt.Location() != time.UTC {
		t.Errorf("OccurredAt location = %v, want UTC", e.OccurredAt.Location())
	}

	limited, err := s.ListUnpublished(ctx, 1)
	if err != nil || len(limited) != 1 || limited[0].ID != earlier {
		t.Fatalf("limit 1: events=%+v err=%v", limited, err)
	}

	if err := s.MarkPublished(ctx, []int64{earlier}); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}
	events, err = s.ListUnpublished(ctx, 10)
	if err != nil || len(events) != 1 || events[0].ID != later {
		t.Fatalf("after mark: events=%+v err=%v, want only %d", events, err, later)
	}

	// Re-marking (a redelivery after a lost acknowledgement) keeps the
	// original published_at.
	var first time.Time
	if err := pool.QueryRow(ctx, `SELECT published_at FROM outbox_events WHERE id = $1`, earlier).Scan(&first); err != nil {
		t.Fatalf("read published_at: %v", err)
	}
	if err := s.MarkPublished(ctx, []int64{earlier, later}); err != nil {
		t.Fatalf("MarkPublished again: %v", err)
	}
	var again time.Time
	if err := pool.QueryRow(ctx, `SELECT published_at FROM outbox_events WHERE id = $1`, earlier).Scan(&again); err != nil {
		t.Fatalf("read published_at: %v", err)
	}
	if !again.Equal(first) {
		t.Errorf("published_at changed on re-mark: %v -> %v", first, again)
	}
	events, err = s.ListUnpublished(ctx, 10)
	if err != nil || len(events) != 0 {
		t.Fatalf("after marking all: events=%+v err=%v", events, err)
	}
}
