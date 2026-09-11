-- name: GetUserByEmail :one
SELECT id, email, password_hash, role, created_at
FROM users
WHERE email = $1;

-- name: InsertUserIfAbsent :one
-- Idempotent seeding: returns no row if the email already exists, so the
-- caller (the seed-users command) can skip logging a duplicate creation.
INSERT INTO users (email, password_hash, role)
VALUES ($1, $2, $3)
ON CONFLICT (email) DO NOTHING
RETURNING id, email, password_hash, role, created_at;
