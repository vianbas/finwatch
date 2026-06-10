package alerts

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/vianbas/finwatch/apps/api/internal/rules"
	"github.com/vianbas/finwatch/apps/api/internal/transactions"
)

// Pagination bounds for listing alerts.
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 200
)

// Cursor identifies a position in the keyset-paginated list, ordered by
// (CreatedAt desc, ID desc).
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// PageQuery requests a page of alerts. Status "" means all statuses; a nil After
// means the first page.
type PageQuery struct {
	Limit  int
	Status Status
	After  *Cursor
}

// Page is a single page of alerts.
type Page struct {
	Items []Alert
	Next  *Cursor
}

// Raise describes an alert to create from a rule match.
type Raise struct {
	RuleID        string
	TransactionID string
	Severity      string
}

// Repository is the persistence port for alerts.
type Repository interface {
	// Raise inserts an alert and its alert.raised outbox event atomically. The
	// bool is false (with a nil error) when the alert already existed, so no
	// duplicate event was emitted.
	Raise(ctx context.Context, r Raise) (Alert, bool, error)
	Get(ctx context.Context, id string) (Alert, error)
	List(ctx context.Context, q PageQuery) (Page, error)
	// Transition conditionally moves an alert from one status to another,
	// returning ErrNotFound when no row matched (missing id or stale status).
	Transition(ctx context.Context, id string, from, to Status) (Alert, error)
}

// Service orchestrates rule evaluation and the alert lifecycle.
type Service struct {
	repo  Repository
	rules rules.Repository
	log   *slog.Logger
}

// NewService constructs a Service.
func NewService(repo Repository, rulesRepo rules.Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, rules: rulesRepo, log: log}
}

// TransactionIngested implements transactions.Observer: it evaluates enabled
// rules against the ingested transaction and raises an alert per match. Raising
// is idempotent, so re-ingesting the same transaction does not duplicate alerts.
func (s *Service) TransactionIngested(ctx context.Context, t transactions.Transaction) error {
	enabled, err := s.rules.ListEnabled(ctx)
	if err != nil {
		return err
	}
	for _, m := range rules.Evaluate(t, enabled) {
		alert, created, err := s.repo.Raise(ctx, Raise{
			RuleID:        m.RuleID,
			TransactionID: t.ID,
			Severity:      m.Severity,
		})
		if err != nil {
			return err
		}
		if created {
			s.log.InfoContext(ctx, "alert raised",
				slog.String("alert_id", alert.ID),
				slog.String("rule_id", m.RuleID),
				slog.String("transaction_id", t.ID),
				slog.String("severity", m.Severity),
			)
		}
	}
	return nil
}

// List returns a page of alerts, clamping the limit to a safe range.
func (s *Service) List(ctx context.Context, q PageQuery) (Page, error) {
	if q.Limit <= 0 {
		q.Limit = DefaultPageLimit
	}
	if q.Limit > MaxPageLimit {
		q.Limit = MaxPageLimit
	}
	return s.repo.List(ctx, q)
}

// Get returns a single alert or ErrNotFound.
func (s *Service) Get(ctx context.Context, id string) (Alert, error) {
	return s.repo.Get(ctx, id)
}

// Acknowledge moves an alert open -> acknowledged.
func (s *Service) Acknowledge(ctx context.Context, id string) (Alert, error) {
	return s.transition(ctx, id, StatusOpen, StatusAcknowledged)
}

// Resolve moves an alert acknowledged -> resolved.
func (s *Service) Resolve(ctx context.Context, id string) (Alert, error) {
	return s.transition(ctx, id, StatusAcknowledged, StatusResolved)
}

// transition applies a single lifecycle step. It is idempotent (a no-op when the
// alert is already in the target state) and returns ErrInvalidTransition for
// illegal steps. The conditional repository update guards against lost updates
// under concurrency.
func (s *Service) transition(ctx context.Context, id string, from, to Status) (Alert, error) {
	current, err := s.repo.Get(ctx, id)
	if err != nil {
		return Alert{}, err
	}
	if current.Status == to {
		return current, nil // already there
	}
	if current.Status != from {
		return Alert{}, ErrInvalidTransition
	}

	updated, err := s.repo.Transition(ctx, id, from, to)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// The status changed between Get and Transition (a concurrent
			// caller). Re-read to decide between idempotent success and conflict.
			latest, getErr := s.repo.Get(ctx, id)
			if getErr != nil {
				return Alert{}, getErr
			}
			if latest.Status == to {
				return latest, nil
			}
			return Alert{}, ErrInvalidTransition
		}
		return Alert{}, err
	}

	s.log.InfoContext(ctx, "alert transition",
		slog.String("alert_id", id),
		slog.String("from", string(from)),
		slog.String("to", string(to)),
	)
	return updated, nil
}
