-- name: ListUnpublishedOutboxEvents :many
-- Oldest first, served by the outbox_events_unpublished_idx partial index.
SELECT id, event_type, payload, occurred_at
FROM outbox_events
WHERE published_at IS NULL
ORDER BY occurred_at, id
LIMIT $1;

-- name: MarkOutboxEventsPublished :exec
UPDATE outbox_events
SET published_at = now()
WHERE id = ANY(sqlc.arg(ids)::bigint[])
  AND published_at IS NULL;
