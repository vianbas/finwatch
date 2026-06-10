-- Reverse of 0001_init.
DROP INDEX IF EXISTS outbox_events_unpublished_idx;
DROP TABLE IF EXISTS outbox_events;
