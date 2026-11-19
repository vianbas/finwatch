// Package store is the PostgreSQL implementation of rules.Repository.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

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
	return toDomainAll(rows), nil
}

// List returns every rule, enabled or not, oldest first.
func (s *Store) List(ctx context.Context) ([]rules.Rule, error) {
	rows, err := db.New(s.pool).ListRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list all rules: %w", err)
	}
	return toDomainAll(rows), nil
}

// SetEnabled enables or disables a rule and returns it, or rules.ErrNotFound.
func (s *Store) SetEnabled(ctx context.Context, id string, enabled bool) (rules.Rule, error) {
	uid, err := pgconv.ParseUUID(id)
	if err != nil {
		return rules.Rule{}, rules.ErrNotFound // an unparseable id cannot exist
	}
	row, err := db.New(s.pool).SetRuleEnabled(ctx, db.SetRuleEnabledParams{ID: uid, Enabled: enabled})
	if errors.Is(err, pgx.ErrNoRows) {
		return rules.Rule{}, rules.ErrNotFound
	}
	if err != nil {
		return rules.Rule{}, fmt.Errorf("store: set rule enabled: %w", err)
	}
	return toDomain(row), nil
}

func toDomainAll(rows []db.Rule) []rules.Rule {
	out := make([]rules.Rule, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDomain(r))
	}
	return out
}

func toDomain(r db.Rule) rules.Rule {
	return rules.Rule{
		ID:        pgconv.UUIDString(r.ID),
		Name:      r.Name,
		Field:     rules.Field(r.Field),
		Operator:  rules.Operator(r.Operator),
		Value:     r.Value,
		Severity:  r.Severity,
		Enabled:   r.Enabled,
		CreatedAt: r.CreatedAt.Time.UTC(),
	}
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
