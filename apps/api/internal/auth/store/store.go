// Package store is the PostgreSQL implementation of auth.UserStore.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vianbas/finwatch/apps/api/internal/auth"
	"github.com/vianbas/finwatch/apps/api/internal/platform/postgres/db"
	"github.com/vianbas/finwatch/apps/api/internal/platform/postgres/pgconv"
)

// Store persists users in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// New constructs a Store.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// GetUserByEmail implements auth.UserStore.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (auth.User, error) {
	row, err := db.New(s.pool).GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("store: get user by email: %w", err)
	}
	return toDomain(row), nil
}

// InsertUserIfAbsent creates a user with an already-hashed password. created
// is false (with a nil error) if the email already exists, so the
// seed-users command can skip logging a duplicate.
func (s *Store) InsertUserIfAbsent(ctx context.Context, email, passwordHash string, role auth.Role) (auth.User, bool, error) {
	row, err := db.New(s.pool).InsertUserIfAbsent(ctx, db.InsertUserIfAbsentParams{
		Email:        email,
		PasswordHash: passwordHash,
		Role:         string(role),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, false, nil
	}
	if err != nil {
		return auth.User{}, false, fmt.Errorf("store: insert user: %w", err)
	}
	return toDomain(row), true, nil
}

func toDomain(r db.User) auth.User {
	return auth.User{
		ID:           pgconv.UUIDString(r.ID),
		Email:        r.Email,
		PasswordHash: r.PasswordHash,
		Role:         auth.Role(r.Role),
	}
}
