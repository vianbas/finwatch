// Package store is the PostgreSQL implementation of rules.Repository.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vianbas/finwatch/apps/api/internal/platform/postgres/db"
	"github.com/vianbas/finwatch/apps/api/internal/platform/postgres/pgconv"
	"github.com/vianbas/finwatch/apps/api/internal/rules"
)

// Store loads and seeds rules.
type Store struct {
	pool *pgxpool.Pool
}

// New constructs a Store.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// ListEnabled returns all enabled rules, oldest first.
func (s *Store) ListEnabled(ctx context.Context) ([]rules.Rule, error) {
	rows, err := db.New(s.pool).ListEnabledRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list rules: %w", err)
	}
	out := make([]rules.Rule, 0, len(rows))
	for _, r := range rows {
		out = append(out, rules.Rule{
			ID:       pgconv.UUIDString(r.ID),
			Name:     r.Name,
			Field:    rules.Field(r.Field),
			Operator: rules.Operator(r.Operator),
			Value:    r.Value,
			Severity: r.Severity,
			Enabled:  r.Enabled,
		})
	}
	return out, nil
}

// EnsureDefaults inserts the generic default rules if they are not present. It
// is idempotent (insert on conflict do nothing by name).
func (s *Store) EnsureDefaults(ctx context.Context) error {
	q := db.New(s.pool)
	for _, r := range rules.DefaultRules {
		if err := q.InsertRuleIfAbsent(ctx, db.InsertRuleIfAbsentParams{
			Name:     r.Name,
			Field:    string(r.Field),
			Operator: string(r.Operator),
			Value:    r.Value,
			Severity: r.Severity,
			Enabled:  r.Enabled,
		}); err != nil {
			return fmt.Errorf("store: ensure rule %q: %w", r.Name, err)
		}
	}
	return nil
}
