-- name: InsertTransaction :one
INSERT INTO transactions (account_id, amount_minor, currency, direction, status, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, account_id, amount_minor, currency, direction, status, occurred_at, created_at;

-- name: InsertOutboxEvent :exec
INSERT INTO outbox_events (aggregate, event_type, payload, occurred_at)
VALUES ($1, $2, $3, $4);

-- name: ListTransactionsFirstPage :many
SELECT id, account_id, amount_minor, currency, direction, status, occurred_at, created_at
FROM transactions
ORDER BY occurred_at DESC, id DESC
LIMIT $1;

-- name: ListTransactionsAfterCursor :many
-- Expanded keyset predicate (not a row-value comparison) so sqlc infers the
-- correct column types: occurred_at is timestamptz, id is uuid.
SELECT id, account_id, amount_minor, currency, direction, status, occurred_at, created_at
FROM transactions
WHERE occurred_at < sqlc.arg(cursor_occurred_at)
   OR (occurred_at = sqlc.arg(cursor_occurred_at) AND id < sqlc.arg(cursor_id))
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg(page_limit);
