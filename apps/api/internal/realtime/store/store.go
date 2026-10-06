// Package store implements the realtime relay's Outbox over PostgreSQL.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vianbas/finwatch/apps/api/internal/platform/postgres/db"
	"github.com/vianbas/finwatch/apps/api/internal/realtime"
)

// Store reads and marks rows in outbox_events.
type Store struct {
	q *db.Queries
}

// New constructs a Store over the given pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{q: db.New(pool)}
}

// ListUnpublished implements realtime.Outbox.
func (s *Store) ListUnpublished(ctx context.Context, limit int) ([]realtime.Event, error) {
	rows, err := s.q.ListUnpublishedOutboxEvents(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("store: list unpublished outbox events: %w", err)
	}
	events := make([]realtime.Event, len(rows))
	for i, row := range rows {
		events[i] = realtime.Event{
			ID:         row.ID,
			Type:       row.EventType,
			Payload:    row.Payload,
			OccurredAt: row.OccurredAt.Time.UTC(),
		}
	}
	return events, nil
}

// MarkPublished implements realtime.Outbox.
func (s *Store) MarkPublished(ctx context.Context, ids []int64) error {
	if err := s.q.MarkOutboxEventsPublished(ctx, ids); err != nil {
		return fmt.Errorf("store: mark outbox events published: %w", err)
	}
	return nil
}
