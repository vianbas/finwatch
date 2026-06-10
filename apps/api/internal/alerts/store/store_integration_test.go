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

	"github.com/vianbas/finwatch/apps/api/internal/alerts"
)

// These tests exercise real PostgreSQL behaviour. They run only when
// FINWATCH_TEST_DATABASE_URL points at a disposable database, e.g.:
//
//	make dev   # starts Postgres on localhost:5432
//	FINWATCH_TEST_DATABASE_URL='postgres://finwatch:finwatch_dev_password@localhost:5432/finwatch?sslmode=disable' \
//	  go test ./internal/alerts/store/...
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

// insertRule seeds a single enabled rule and returns its UUID string.
func insertRule(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO rules (name, field, operator, value, severity, enabled)
		VALUES ('test-rule', 'amount_minor', 'gte', '0', 'high', true)
		RETURNING id::text
	`).Scan(&id)
	if err != nil {
		t.Fatalf("insert rule: %v", err)
	}
	return id
}

// insertTransaction seeds a single posted transaction and returns its UUID string.
func insertTransaction(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO transactions (account_id, amount_minor, currency, direction, status, occurred_at)
		VALUES ('ACC-1', 1000, 'USD', 'debit', 'posted', now())
		RETURNING id::text
	`).Scan(&id)
	if err != nil {
		t.Fatalf("insert transaction: %v", err)
	}
	return id
}

func TestStore_Raise_AtomicAndCreated(t *testing.T) {
	pool := newTestPool(t)
	s := New(pool)
	ctx := context.Background()

	ruleID := insertRule(t, pool)
	txnID := insertTransaction(t, pool)

	got, created, err := s.Raise(ctx, alerts.Raise{RuleID: ruleID, TransactionID: txnID, Severity: "high"})
	if err != nil {
		t.Fatalf("raise: %v", err)
	}
	if !created {
		t.Fatal("want created=true on first raise")
	}
	if got.ID == "" {
		t.Fatal("want a generated alert id")
	}
	if got.Status != alerts.StatusOpen {
		t.Errorf("status=%q, want open", got.Status)
	}

	// One alert + one outbox event must have been written.
	var alertCount, outboxCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM alerts`).Scan(&alertCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE event_type = 'alert.raised'`).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if alertCount != 1 || outboxCount != 1 {
		t.Fatalf("counts: alerts=%d outbox=%d, want 1/1", alertCount, outboxCount)
	}
}

func TestStore_Raise_Idempotent_DuplicateIgnored(t *testing.T) {
	pool := newTestPool(t)
	s := New(pool)
	ctx := context.Background()

	ruleID := insertRule(t, pool)
	txnID := insertTransaction(t, pool)

	if _, _, err := s.Raise(ctx, alerts.Raise{RuleID: ruleID, TransactionID: txnID, Severity: "high"}); err != nil {
		t.Fatalf("first raise: %v", err)
	}

	_, created, err := s.Raise(ctx, alerts.Raise{RuleID: ruleID, TransactionID: txnID, Severity: "high"})
	if err != nil {
		t.Fatalf("second raise: %v", err)
	}
	if created {
		t.Fatal("want created=false on duplicate raise")
	}

	// Still only one alert and one outbox event; no duplicates.
	var alertCount, outboxCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM alerts`).Scan(&alertCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events`).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if alertCount != 1 || outboxCount != 1 {
		t.Fatalf("counts: alerts=%d outbox=%d, want 1/1 after dedup", alertCount, outboxCount)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	pool := newTestPool(t)
	s := New(pool)
	_, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err != alerts.ErrNotFound {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestStore_Transition_ConditionalUpdate(t *testing.T) {
	pool := newTestPool(t)
	s := New(pool)
	ctx := context.Background()

	ruleID := insertRule(t, pool)
	txnID := insertTransaction(t, pool)
	raised, _, err := s.Raise(ctx, alerts.Raise{RuleID: ruleID, TransactionID: txnID, Severity: "high"})
	if err != nil {
		t.Fatalf("raise: %v", err)
	}

	// open → acknowledged must succeed.
	acked, err := s.Transition(ctx, raised.ID, alerts.StatusOpen, alerts.StatusAcknowledged)
	if err != nil {
		t.Fatalf("transition open→acknowledged: %v", err)
	}
	if acked.Status != alerts.StatusAcknowledged {
		t.Errorf("status=%q, want acknowledged", acked.Status)
	}

	// Stale conditional update (wrong from-status) must return ErrNotFound.
	_, err = s.Transition(ctx, raised.ID, alerts.StatusOpen, alerts.StatusAcknowledged)
	if err != alerts.ErrNotFound {
		t.Fatalf("stale transition: got %v, want ErrNotFound", err)
	}

	// acknowledged → resolved must succeed.
	resolved, err := s.Transition(ctx, raised.ID, alerts.StatusAcknowledged, alerts.StatusResolved)
	if err != nil {
		t.Fatalf("transition acknowledged→resolved: %v", err)
	}
	if resolved.Status != alerts.StatusResolved {
		t.Errorf("status=%q, want resolved", resolved.Status)
	}
}

func TestStore_List_KeysetPagination_WithStatusFilter(t *testing.T) {
	pool := newTestPool(t)
	s := New(pool)
	ctx := context.Background()

	ruleID := insertRule(t, pool)

	// Insert 4 transactions and raise one alert per transaction.
	alertIDs := make([]string, 4)
	for i := 0; i < 4; i++ {
		txnID := insertTransaction(t, pool)
		a, _, err := s.Raise(ctx, alerts.Raise{RuleID: ruleID, TransactionID: txnID, Severity: "high"})
		if err != nil {
			t.Fatalf("raise %d: %v", i, err)
		}
		alertIDs[i] = a.ID
		// Small sleep so created_at differs between rows for deterministic ordering.
		time.Sleep(2 * time.Millisecond)
	}

	// Acknowledge the first two alerts.
	for _, id := range alertIDs[:2] {
		if _, err := s.Transition(ctx, id, alerts.StatusOpen, alerts.StatusAcknowledged); err != nil {
			t.Fatalf("acknowledge %s: %v", id, err)
		}
	}

	// Status filter: only open alerts (2 expected).
	openPage, err := s.List(ctx, alerts.PageQuery{Status: alerts.StatusOpen, Limit: 10})
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if len(openPage.Items) != 2 {
		t.Errorf("open alerts: got %d, want 2", len(openPage.Items))
	}
	for _, a := range openPage.Items {
		if a.Status != alerts.StatusOpen {
			t.Errorf("expected open, got %q", a.Status)
		}
	}

	// Keyset pagination: fetch all 4 in pages of 2, verify no duplicates and
	// descending order.
	seen := map[string]bool{}
	var cursor *alerts.Cursor
	pages := 0
	var prev time.Time
	for {
		page, err := s.List(ctx, alerts.PageQuery{Limit: 2, After: cursor})
		if err != nil {
			t.Fatalf("list page %d: %v", pages, err)
		}
		for _, a := range page.Items {
			if seen[a.ID] {
				t.Fatalf("duplicate alert id across pages: %s", a.ID)
			}
			seen[a.ID] = true
			if !prev.IsZero() && a.CreatedAt.After(prev) {
				t.Fatalf("results not in descending order")
			}
			prev = a.CreatedAt
		}
		pages++
		if page.Next == nil {
			break
		}
		cursor = page.Next
		if pages > 10 {
			t.Fatal("pagination did not terminate")
		}
	}
	if len(seen) != 4 {
		t.Fatalf("saw %d unique alerts, want 4", len(seen))
	}
}
