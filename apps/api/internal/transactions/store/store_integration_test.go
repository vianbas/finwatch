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

	"github.com/vianbas/finwatch/apps/api/internal/transactions"
)

// These tests exercise real PostgreSQL behaviour. They run only when
// FINWATCH_TEST_DATABASE_URL points at a disposable database, e.g.:
//
//	make dev   # starts Postgres on localhost:5432
//	FINWATCH_TEST_DATABASE_URL='postgres://finwatch:finwatch_dev_password@localhost:5432/finwatch?sslmode=disable' \
//	  go test ./internal/transactions/store/...
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

// resetSchema drops the feature tables and re-applies all up migrations.
func resetSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS transactions, outbox_events CASCADE`); err != nil {
		t.Fatalf("drop tables: %v", err)
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
		sqlBytes, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
}

func mustNew(t *testing.T, account string, amount int64, currency string, dir transactions.Direction, occurred time.Time) transactions.Transaction {
	t.Helper()
	tx, err := transactions.New(account, amount, currency, dir, occurred)
	if err != nil {
		t.Fatalf("build transaction: %v", err)
	}
	return tx
}

func TestStore_Save_WritesTransactionAndOutboxAtomically(t *testing.T) {
	pool := newTestPool(t)
	s := New(pool)
	ctx := context.Background()

	occurred := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	saved, err := s.Save(ctx, mustNew(t, "ACC-1", 2500, "USD", transactions.DirectionDebit, occurred))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("expected a generated id")
	}
	if !saved.OccurredAt.Equal(occurred) {
		t.Errorf("occurredAt = %v, want %v", saved.OccurredAt, occurred)
	}

	var txCount, outboxCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM transactions`).Scan(&txCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM outbox_events WHERE event_type = 'transaction.observed'`).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if txCount != 1 || outboxCount != 1 {
		t.Fatalf("counts transactions=%d outbox=%d, want 1/1", txCount, outboxCount)
	}

	var payload []byte
	if err := pool.QueryRow(ctx, `SELECT payload FROM outbox_events LIMIT 1`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), saved.ID) {
		t.Errorf("outbox payload missing transaction id: %s", payload)
	}
}

func TestStore_List_KeysetPagination(t *testing.T) {
	pool := newTestPool(t)
	s := New(pool)
	ctx := context.Background()

	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	const total = 5
	for i := 0; i < total; i++ {
		occ := base.Add(time.Duration(i) * time.Hour)
		if _, err := s.Save(ctx, mustNew(t, "ACC-1", int64(100*(i+1)), "USD", transactions.DirectionCredit, occ)); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}

	seen := map[string]bool{}
	var cursor *transactions.Cursor
	prev := time.Now().Add(24 * time.Hour) // start above any occurred_at
	pages := 0
	for {
		page, err := s.List(ctx, transactions.PageQuery{Limit: 2, After: cursor})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		for _, it := range page.Items {
			if seen[it.ID] {
				t.Fatalf("duplicate id across pages: %s", it.ID)
			}
			seen[it.ID] = true
			if it.OccurredAt.After(prev) {
				t.Fatalf("results not in descending order: %v after %v", it.OccurredAt, prev)
			}
			prev = it.OccurredAt
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
	if len(seen) != total {
		t.Fatalf("saw %d unique transactions, want %d", len(seen), total)
	}
}
