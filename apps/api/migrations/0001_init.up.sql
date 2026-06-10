-- 0001_init: baseline schema.
--
-- This migration establishes the transactional outbox table that underpins the
-- real-time architecture (see docs/adr/0003-postgresql-outbox.md). It is schema
-- only: no producer writes to it and no relay reads from it yet. Feature tables
-- (transactions, alerts, rules) are introduced in their own issues.
--
-- Conventions:
--   * Monetary amounts are stored as BIGINT minor units, never floating point.
--   * Timestamps are stored in UTC (timestamptz) and serialised as RFC 3339.

CREATE TABLE outbox_events (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    aggregate     TEXT        NOT NULL,
    event_type    TEXT        NOT NULL,
    payload       JSONB       NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);

-- Supports the future relay's "fetch unpublished, oldest first" scan.
CREATE INDEX outbox_events_unpublished_idx
    ON outbox_events (occurred_at)
    WHERE published_at IS NULL;
