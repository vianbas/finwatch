// Package postgres provides thin construction helpers around a pgx connection
// pool. It intentionally contains no query logic; data access for features is
// generated with sqlc once schemas and queries exist (see apps/api/migrations
// and apps/api/sqlc.yaml).
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool constructs a pgx connection pool from a connection string. The pool
// establishes connections lazily, so this call does not require the database to
// be reachable yet; readiness is verified separately via Pool.Ping.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}
	return pool, nil
}
