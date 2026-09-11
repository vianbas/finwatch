package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
)

// These tests exercise real PostgreSQL behaviour. They run only when
// FINWATCH_TEST_DATABASE_URL points at a disposable database, e.g.:
//
//	make dev   # starts Postgres on localhost:5432
//	FINWATCH_TEST_DATABASE_URL='postgres://finwatch:finwatch_dev_password@localhost:5432/finwatch?sslmode=disable' \
//	  go test ./internal/auth/store/...
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

func TestStore_InsertUserIfAbsent_CreatesUser(t *testing.T) {
	pool := newTestPool(t)
	s := New(pool)
	ctx := context.Background()

	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	got, created, err := s.InsertUserIfAbsent(ctx, "insert-user@example.com", hash, auth.RoleOperator)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if !created {
		t.Fatal("want created=true on first insert")
	}
	if got.ID == "" {
		t.Fatal("want a generated user id")
	}
	if got.Email != "insert-user@example.com" {
		t.Errorf("email=%q, want insert-user@example.com", got.Email)
	}
	if got.PasswordHash != hash {
		t.Errorf("password hash did not round-trip")
	}
	if got.Role != auth.RoleOperator {
		t.Errorf("role=%q, want operator", got.Role)
	}
}

func TestStore_InsertUserIfAbsent_DuplicateEmailNotCreated(t *testing.T) {
	pool := newTestPool(t)
	s := New(pool)
	ctx := context.Background()

	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if _, _, err := s.InsertUserIfAbsent(ctx, "dup-user@example.com", hash, auth.RoleOperator); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	_, created, err := s.InsertUserIfAbsent(ctx, "dup-user@example.com", hash, auth.RoleAdmin)
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if created {
		t.Fatal("want created=false on duplicate email")
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE email = 'dup-user@example.com'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("users with that email: got %d, want 1", count)
	}
}

func TestStore_GetUserByEmail_ReturnsStoredUser(t *testing.T) {
	pool := newTestPool(t)
	s := New(pool)
	ctx := context.Background()

	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	inserted, _, err := s.InsertUserIfAbsent(ctx, "get-user@example.com", hash, auth.RoleAdmin)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	got, err := s.GetUserByEmail(ctx, "get-user@example.com")
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}
	if got.ID != inserted.ID {
		t.Errorf("id=%q, want %q", got.ID, inserted.ID)
	}
	if got.Email != "get-user@example.com" {
		t.Errorf("email=%q, want get-user@example.com", got.Email)
	}
	if got.PasswordHash != hash {
		t.Errorf("password hash did not round-trip")
	}
	if got.Role != auth.RoleAdmin {
		t.Errorf("role=%q, want admin", got.Role)
	}
}

func TestStore_GetUserByEmail_UnknownEmailReturnsErrUserNotFound(t *testing.T) {
	pool := newTestPool(t)
	s := New(pool)

	_, err := s.GetUserByEmail(context.Background(), "unknown-user@example.com")
	if !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("got %v, want ErrUserNotFound", err)
	}
}
