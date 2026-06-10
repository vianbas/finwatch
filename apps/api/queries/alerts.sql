-- name: InsertAlertIfAbsent :one
-- Idempotent raise: a given (rule, transaction) yields at most one alert. On
-- conflict no row is returned, so the caller can skip emitting a duplicate event.
INSERT INTO alerts (rule_id, transaction_id, severity)
VALUES ($1, $2, $3)
ON CONFLICT (rule_id, transaction_id) DO NOTHING
RETURNING id, rule_id, transaction_id, status, severity, created_at, updated_at;

-- name: GetAlert :one
SELECT id, rule_id, transaction_id, status, severity, created_at, updated_at
FROM alerts
WHERE id = $1;

-- name: UpdateAlertStatus :one
-- Conditional update guards against lost updates: it only transitions when the
-- current status matches. Zero rows means a stale/!concurrent transition.
UPDATE alerts
SET status = sqlc.arg(new_status), updated_at = now()
WHERE id = sqlc.arg(id) AND status = sqlc.arg(current_status)
RETURNING id, rule_id, transaction_id, status, severity, created_at, updated_at;

-- name: ListAlertsFirstPage :many
SELECT id, rule_id, transaction_id, status, severity, created_at, updated_at
FROM alerts
WHERE (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: ListAlertsAfterCursor :many
SELECT id, rule_id, transaction_id, status, severity, created_at, updated_at
FROM alerts
WHERE (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
  AND (created_at < sqlc.arg(cursor_created_at)
       OR (created_at = sqlc.arg(cursor_created_at) AND id < sqlc.arg(cursor_id)))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);
