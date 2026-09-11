-- 0004_users: user accounts for JWT authentication and RBAC.
--
-- Conventions match earlier migrations: UUID ids via gen_random_uuid,
-- TIMESTAMPTZ in UTC, small constrained value sets via CHECK. Roles are
-- intentionally limited to operator and admin for this issue.

CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    role          TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT users_role_valid CHECK (role IN ('operator', 'admin'))
);
