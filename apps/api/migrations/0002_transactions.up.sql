-- 0002_transactions: synthetic transaction records.
--
-- Stores ingested synthetic transactions. Conventions:
--   * id is a server-generated UUID (gen_random_uuid is built into PostgreSQL
--     13+; no extension required).
--   * amount_minor is BIGINT integer minor units (e.g. cents) — never float.
--   * occurred_at is TIMESTAMPTZ stored in UTC; serialised as RFC 3339.
--   * direction and status are constrained to small, generic value sets.

CREATE TABLE transactions (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id    TEXT        NOT NULL,
    amount_minor  BIGINT      NOT NULL,
    currency      CHAR(3)     NOT NULL,
    direction     TEXT        NOT NULL,
    status        TEXT        NOT NULL,
    occurred_at   TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT transactions_amount_minor_positive CHECK (amount_minor > 0),
    CONSTRAINT transactions_direction_valid CHECK (direction IN ('debit', 'credit')),
    CONSTRAINT transactions_currency_format CHECK (currency ~ '^[A-Z]{3}$')
);

-- Supports keyset (cursor) pagination ordered by most-recent first.
CREATE INDEX transactions_occurred_at_id_idx
    ON transactions (occurred_at DESC, id DESC);
