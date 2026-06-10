# Database migrations

Versioned SQL migrations applied with [`golang-migrate`](https://github.com/golang-migrate/migrate).

## Conventions

- Files are named `<version>_<name>.up.sql` and `<version>_<name>.down.sql`.
- `version` is a zero-padded, monotonically increasing integer.
- Every `up` migration has a matching `down` migration.
- Migrations are immutable once merged; corrections are made in a new migration.
- Monetary values use `BIGINT` minor units; never floating point.
- Timestamps use `TIMESTAMPTZ` stored in UTC.

## Applying

Migrations are not run automatically by the API process. Apply them with the
`migrate` CLI against `DATABASE_URL`, for example:

```sh
migrate -path apps/api/migrations -database "$DATABASE_URL" up
```
