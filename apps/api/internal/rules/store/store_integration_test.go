package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vianbas/finwatch/apps/api/internal/rules"
)

// These tests exercise real PostgreSQL behaviour. They run only when
// FINWATCH_TEST_DATABASE_URL points at a disposable database, e.g.:
//
//	make dev   # starts Postgres on localhost:5432
//	FINWATCH_TEST_DATABASE_URL='postgres://finwatch:finwatch_dev_password@localhost:5432/finwatch?sslmode=disable' \
//	  go test ./internal/rules/store/...
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

func TestStore_ListAndSetEnabled(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	s := New(pool)

	if err := s.EnsureDefaults(ctx); err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}
	all, err := s.List(ctx)
	if err != nil || len(all) != len(rules.DefaultRules) {
		t.Fatalf("List = %d rules (err %v), want %d", len(all), err, len(rules.DefaultRules))
	}
	target := all[0]
	if !target.Enabled || target.CreatedAt.IsZero() || target.CreatedAt.Location() != time.UTC {
		t.Fatalf("unexpected seeded rule %+v", target)
	}

	got, err := s.SetEnabled(ctx, target.ID, false)
	if err != nil {
		t.Fatalf("SetEnabled(false): %v", err)
	}
	if got.ID != target.ID || got.Enabled || got.Name != target.Name {
		t.Errorf("SetEnabled returned %+v, want %s disabled", got, target.ID)
	}

	// Disabled rules stay listed but drop out of evaluation.
	all, err = s.List(ctx)
	if err != nil || len(all) != len(rules.DefaultRules) {
		t.Fatalf("List after disable = %d (err %v)", len(all), err)
	}
	enabled, err := s.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("ListEnabled: %v", err)
	}
	for _, r := range enabled {
		if r.ID == target.ID {
			t.Errorf("disabled rule %s still returned by ListEnabled", target.ID)
		}
	}

	// Idempotent, and reversible.
	if _, err := s.SetEnabled(ctx, target.ID, false); err != nil {
		t.Fatalf("SetEnabled(false) again: %v", err)
	}
	if got, err := s.SetEnabled(ctx, target.ID, true); err != nil || !got.Enabled {
		t.Fatalf("SetEnabled(true) = %+v, %v", got, err)
	}
}

func TestStore_SetEnabledUnknownRule(t *testing.T) {
	pool := newTestPool(t)
	s := New(pool)
	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid"} {
		if _, err := s.SetEnabled(context.Background(), id, false); !errors.Is(err, rules.ErrNotFound) {
			t.Errorf("SetEnabled(%q) err = %v, want ErrNotFound", id, err)
		}
	}
}
