// Package store is the PostgreSQL implementation of alerts.Repository.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vianbas/finwatch/apps/api/internal/alerts"
	"github.com/vianbas/finwatch/apps/api/internal/platform/postgres/db"
	"github.com/vianbas/finwatch/apps/api/internal/platform/postgres/pgconv"
)

const (
	outboxAggregate      = "alert"
	eventTypeAlertRaised = "alert.raised"
)

// Store persists alerts in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// New constructs a Store.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// raisedData is the outbox payload for alert.raised. It matches
// AlertRaisedData in contracts/asyncapi.yaml.
type raisedData struct {
	AlertID       string `json:"alertId"`
	RuleID        string `json:"ruleId"`
	TransactionID string `json:"transactionId"`
	Severity      string `json:"severity"`
	Status        string `json:"status"`
	RaisedAt      string `json:"raisedAt"`
}

// Raise inserts the alert and its alert.raised outbox event in one transaction.
// If the alert already exists (same rule + transaction), nothing is written and
// created is false.
func (s *Store) Raise(ctx context.Context, r alerts.Raise) (alerts.Alert, bool, error) {
	ruleID, err := pgconv.ParseUUID(r.RuleID)
	if err != nil {
		return alerts.Alert{}, false, fmt.Errorf("store: rule id: %w", err)
	}
	txnID, err := pgconv.ParseUUID(r.TransactionID)
	if err != nil {
		return alerts.Alert{}, false, fmt.Errorf("store: transaction id: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return alerts.Alert{}, false, fmt.Errorf("store: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	row, err := q.InsertAlertIfAbsent(ctx, db.InsertAlertIfAbsentParams{
		RuleID:        ruleID,
		TransactionID: txnID,
		Severity:      r.Severity,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return alerts.Alert{}, false, nil // already exists; no event emitted
	}
	if err != nil {
		return alerts.Alert{}, false, fmt.Errorf("store: insert alert: %w", err)
	}

	saved := toDomain(row)
	payload, err := json.Marshal(raisedData{
		AlertID:       saved.ID,
		RuleID:        saved.RuleID,
		TransactionID: saved.TransactionID,
		Severity:      saved.Severity,
		Status:        string(saved.Status),
		RaisedAt:      saved.CreatedAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		return alerts.Alert{}, false, fmt.Errorf("store: marshal event: %w", err)
	}
	if err := q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
		Aggregate:  outboxAggregate,
		EventType:  eventTypeAlertRaised,
		Payload:    payload,
		OccurredAt: row.CreatedAt,
	}); err != nil {
		return alerts.Alert{}, false, fmt.Errorf("store: insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return alerts.Alert{}, false, fmt.Errorf("store: commit: %w", err)
	}
	return saved, true, nil
}

// Get returns an alert by id, or alerts.ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (alerts.Alert, error) {
	uid, err := pgconv.ParseUUID(id)
	if err != nil {
		return alerts.Alert{}, alerts.ErrNotFound // an unparseable id cannot exist
	}
	row, err := db.New(s.pool).GetAlert(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return alerts.Alert{}, alerts.ErrNotFound
	}
	if err != nil {
		return alerts.Alert{}, fmt.Errorf("store: get alert: %w", err)
	}
	return toDomain(row), nil
}

// Transition conditionally updates status, returning ErrNotFound when no row
// matched (missing id or a status that no longer equals `from`).
func (s *Store) Transition(ctx context.Context, id string, from, to alerts.Status) (alerts.Alert, error) {
	uid, err := pgconv.ParseUUID(id)
	if err != nil {
		return alerts.Alert{}, alerts.ErrNotFound
	}
	row, err := db.New(s.pool).UpdateAlertStatus(ctx, db.UpdateAlertStatusParams{
		NewStatus:     string(to),
		ID:            uid,
		CurrentStatus: string(from),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return alerts.Alert{}, alerts.ErrNotFound
	}
	if err != nil {
		return alerts.Alert{}, fmt.Errorf("store: update alert status: %w", err)
	}
	return toDomain(row), nil
}

// List returns a page of alerts (most-recent first), optionally filtered by
// status, fetching one extra row to detect a next page.
func (s *Store) List(ctx context.Context, q alerts.PageQuery) (alerts.Page, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = alerts.DefaultPageLimit
	}
	fetch := int32(limit + 1)

	var statusFilter *string
	if q.Status != "" {
		st := string(q.Status)
		statusFilter = &st
	}

	queries := db.New(s.pool)
	var rows []db.Alert
	var err error
	if q.After == nil {
		rows, err = queries.ListAlertsFirstPage(ctx, db.ListAlertsFirstPageParams{
			Status:    statusFilter,
			PageLimit: fetch,
		})
	} else {
		cursorID, perr := pgconv.ParseUUID(q.After.ID)
		if perr != nil {
			return alerts.Page{}, fmt.Errorf("store: cursor id: %w", perr)
		}
		rows, err = queries.ListAlertsAfterCursor(ctx, db.ListAlertsAfterCursorParams{
			Status:          statusFilter,
			CursorCreatedAt: pgconv.Timestamptz(q.After.CreatedAt),
			CursorID:        cursorID,
			PageLimit:       fetch,
		})
	}
	if err != nil {
		return alerts.Page{}, fmt.Errorf("store: list alerts: %w", err)
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	page := alerts.Page{Items: make([]alerts.Alert, 0, len(rows))}
	for _, r := range rows {
		page.Items = append(page.Items, toDomain(r))
	}
	if hasMore && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		page.Next = &alerts.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return page, nil
}

func toDomain(r db.Alert) alerts.Alert {
	return alerts.Alert{
		ID:            pgconv.UUIDString(r.ID),
		RuleID:        pgconv.UUIDString(r.RuleID),
		TransactionID: pgconv.UUIDString(r.TransactionID),
		Status:        alerts.Status(r.Status),
		Severity:      r.Severity,
		CreatedAt:     r.CreatedAt.Time.UTC(),
		UpdatedAt:     r.UpdatedAt.Time.UTC(),
	}
}
