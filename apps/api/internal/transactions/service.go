package transactions

import (
	"context"
	"log/slog"
	"time"
)

// Pagination bounds for listing transactions.
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 200
)

// Cursor identifies a position in the keyset-paginated list, ordered by
// (OccurredAt desc, ID desc).
type Cursor struct {
	OccurredAt time.Time
	ID         string
}

// PageQuery requests a page of transactions. A nil After means the first page.
type PageQuery struct {
	Limit int
	After *Cursor
}

// Page is a single page of results. Next is nil when there are no more rows.
type Page struct {
	Items []Transaction
	Next  *Cursor
}

// Repository is the persistence port for the transactions module. The pgx/sqlc
// implementation lives in the store subpackage; tests use fakes.
type Repository interface {
	// Save persists t together with its outbox event atomically and returns the
	// stored record (with ID and CreatedAt populated).
	Save(ctx context.Context, t Transaction) (Transaction, error)
	// List returns a page of transactions most-recent first.
	List(ctx context.Context, q PageQuery) (Page, error)
}

// Observer is notified after each transaction is persisted. Downstream modules
// (e.g. alerts) implement it to react to ingestion, so the transactions module
// never imports them. It is best-effort: an observer error does not roll back
// the already-committed transaction.
type Observer interface {
	TransactionIngested(ctx context.Context, t Transaction) error
}

// Service orchestrates transaction ingestion and listing.
type Service struct {
	repo     Repository
	gen      *Generator
	log      *slog.Logger
	observer Observer
}

// NewService constructs a Service.
func NewService(repo Repository, gen *Generator, log *slog.Logger) *Service {
	return &Service{repo: repo, gen: gen, log: log}
}

// SetObserver registers an observer notified after each successful save.
func (s *Service) SetObserver(o Observer) {
	s.observer = o
}

// Ingest generates and persists n synthetic transactions. Each transaction and
// its outbox event are written in a single database transaction by the
// repository, so a failure leaves no partial event. It returns the number
// successfully persisted.
func (s *Service) Ingest(ctx context.Context, n int) (int, error) {
	created := 0
	for i := 0; i < n; i++ {
		saved, err := s.repo.Save(ctx, s.gen.Next())
		if err != nil {
			s.log.ErrorContext(ctx, "ingest failed",
				slog.Int("persisted", created), slog.String("error", err.Error()))
			return created, err
		}
		created++

		// Notify observers (e.g. rule evaluation). Best-effort: the transaction
		// is already committed, so an observer failure is logged, not fatal.
		if s.observer != nil {
			if err := s.observer.TransactionIngested(ctx, saved); err != nil {
				s.log.ErrorContext(ctx, "transaction observer failed",
					slog.String("transaction_id", saved.ID), slog.String("error", err.Error()))
			}
		}
	}
	s.log.InfoContext(ctx, "ingested synthetic transactions", slog.Int("count", created))
	return created, nil
}

// List returns a page of transactions, clamping the limit to a safe range.
func (s *Service) List(ctx context.Context, q PageQuery) (Page, error) {
	if q.Limit <= 0 {
		q.Limit = DefaultPageLimit
	}
	if q.Limit > MaxPageLimit {
		q.Limit = MaxPageLimit
	}
	return s.repo.List(ctx, q)
}
