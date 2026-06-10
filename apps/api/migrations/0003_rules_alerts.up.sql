-- 0003_rules_alerts: declarative rules and the alert lifecycle.
--
-- rules are generic, declarative operational rules (field/operator/value) used to
-- evaluate synthetic transactions. alerts records matches and their lifecycle
-- (open -> acknowledged -> resolved). Conventions match earlier migrations:
-- UUID ids via gen_random_uuid, TIMESTAMPTZ in UTC, small constrained value sets.

CREATE TABLE rules (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL UNIQUE,
    field      TEXT        NOT NULL,
    operator   TEXT        NOT NULL,
    value      TEXT        NOT NULL,
    severity   TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT rules_field_valid CHECK (field IN ('amount_minor', 'currency', 'direction')),
    CONSTRAINT rules_operator_valid CHECK (operator IN ('gte', 'lte', 'eq')),
    CONSTRAINT rules_severity_valid CHECK (severity IN ('low', 'medium', 'high', 'critical'))
);

CREATE TABLE alerts (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id        UUID        NOT NULL REFERENCES rules (id),
    transaction_id UUID        NOT NULL REFERENCES transactions (id),
    status         TEXT        NOT NULL DEFAULT 'open',
    severity       TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT alerts_status_valid CHECK (status IN ('open', 'acknowledged', 'resolved')),
    CONSTRAINT alerts_severity_valid CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    -- One alert per (rule, transaction): makes raising idempotent.
    CONSTRAINT alerts_rule_txn_unique UNIQUE (rule_id, transaction_id)
);

-- Supports keyset pagination (most-recent first) and status filtering.
CREATE INDEX alerts_created_at_id_idx ON alerts (created_at DESC, id DESC);
CREATE INDEX alerts_status_idx ON alerts (status);
