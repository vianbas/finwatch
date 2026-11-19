-- name: ListEnabledRules :many
SELECT id, name, field, operator, value, severity, enabled, created_at
FROM rules
WHERE enabled = TRUE
ORDER BY created_at, id;

-- name: InsertRuleIfAbsent :exec
INSERT INTO rules (name, field, operator, value, severity, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (name) DO NOTHING;

-- name: ListRules :many
SELECT id, name, field, operator, value, severity, enabled, created_at
FROM rules
ORDER BY created_at, id;

-- name: SetRuleEnabled :one
UPDATE rules
SET enabled = sqlc.arg(enabled)
WHERE id = sqlc.arg(id)
RETURNING id, name, field, operator, value, severity, enabled, created_at;
