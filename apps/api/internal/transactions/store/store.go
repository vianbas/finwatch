// Package store is the PostgreSQL implementation of transactions.Repository,
// backed by pgx and sqlc-generated queries. It is the only place in the module
// that knows about pgx/pgtype; the domain stays database-agnostic.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vianbas/finwatch/apps/api/internal/platform/postgres/db"
	"github.com/vianbas/finwatch/apps/api/internal/transactions"
)

const (
	outboxAggregate     = "transaction"
	eventTypeTxObserved = "transaction.observed"
)

// Store persists transactions in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// New constructs a Store over the given pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// observedData is the payload stored in the outbox row. It matches
// TransactionObservedData in contracts/asyncapi.yaml.
type observedData struct {
	TransactionID string `json:"transactionId"`
	AccountID     string `json:"accountId"`
	AmountMinor   int64  `json:"amountMinor"`
	Currency      string `json:"currency"`
	Direction     string `json:"direction"`
	Status        string `json:"status"`
	OccurredAt    string `json:"occurredAt"`
}

// Save inserts a transaction and its transaction.observed outbox event in a
// single database transaction, so the event exists if and only if the insert
// committed.
func (s *Store) Save(ctx context.Context, t transactions.Transaction) (transactions.Transaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return transactions.Transaction{}, fmt.Errorf("store: begin tx: %w", err)
	}
	// Rollback is a no-op once the tx has committed.
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	row, err := q.InsertTransaction(ctx, db.InsertTransactionParams{
		AccountID:   t.AccountID,
		AmountMinor: t.AmountMinor,
		Currency:    t.Currency,
		Direction:   string(t.Direction),
		Status:      string(t.Status),
		OccurredAt:  timestamptz(t.OccurredAt),
	})
	if err != nil {
		return transactions.Transaction{}, fmt.Errorf("store: insert transaction: %w", err)
	}

	saved := toDomain(row)

	payload, err := json.Marshal(observedData{
		TransactionID: saved.ID,
		AccountID:     saved.AccountID,
		AmountMinor:   saved.AmountMinor,
		Currency:      saved.Currency,
		Direction:     string(saved.Direction),
		Status:        string(saved.Status),
		OccurredAt:    saved.OccurredAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		return transactions.Transaction{}, fmt.Errorf("store: marshal event: %w", err)
	}

	if err := q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
		Aggregate:  outboxAggregate,
		EventType:  eventTypeTxObserved,
		Payload:    payload,
		OccurredAt: row.OccurredAt,
	}); err != nil {
		return transactions.Transaction{}, fmt.Errorf("store: insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return transactions.Transaction{}, fmt.Errorf("store: commit: %w", err)
	}
	return saved, nil
}

// List returns a page of transactions ordered most-recent first. It fetches one
// extra row to determine whether a next page exists.
func (s *Store) List(ctx context.Context, q transactions.PageQuery) (transactions.Page, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = transactions.DefaultPageLimit
	}
	fetch := int32(limit + 1)

	queries := db.New(s.pool)

	var rows []db.Transaction
	var err error
	if q.After == nil {
		rows, err = queries.ListTransactionsFirstPage(ctx, fetch)
	} else {
		var cursorID pgtype.UUID
		if scanErr := cursorID.Scan(q.After.ID); scanErr != nil {
			return transactions.Page{}, fmt.Errorf("store: invalid cursor id: %w", scanErr)
		}
		rows, err = queries.ListTransactionsAfterCursor(ctx, db.ListTransactionsAfterCursorParams{
			CursorOccurredAt: timestamptz(q.After.OccurredAt),
			CursorID:         cursorID,
			PageLimit:        fetch,
		})
	}
	if err != nil {
		return transactions.Page{}, fmt.Errorf("store: list transactions: %w", err)
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	page := transactions.Page{Items: make([]transactions.Transaction, 0, len(rows))}
	for _, r := range rows {
		page.Items = append(page.Items, toDomain(r))
	}
	if hasMore && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		page.Next = &transactions.Cursor{OccurredAt: last.OccurredAt, ID: last.ID}
	}
	return page, nil
}

func toDomain(r db.Transaction) transactions.Transaction {
	return transactions.Transaction{
		ID:          uuidString(r.ID),
		AccountID:   r.AccountID,
		AmountMinor: r.AmountMinor,
		Currency:    r.Currency,
		Direction:   transactions.Direction(r.Direction),
		Status:      transactions.Status(r.Status),
		OccurredAt:  r.OccurredAt.Time.UTC(),
		CreatedAt:   r.CreatedAt.Time.UTC(),
	}
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

// uuidString renders a pgtype.UUID as its canonical textual form.
func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
