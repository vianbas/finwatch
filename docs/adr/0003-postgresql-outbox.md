# ADR 0003: PostgreSQL transactional outbox + in-process WebSocket hub

- Status: Accepted
- Date: 2026-06-10

## Context

Operators need near-real-time updates (new transactions, raised alerts). We need
events to be delivered reliably and **atomically with the state change** that
produced them, without introducing operational complexity or extra
infrastructure into a reference implementation.

## Decision

Use the **transactional outbox** pattern on PostgreSQL plus an **in-process
WebSocket hub**:

1. A state change and its event row (`outbox_events`) are written in the **same
   database transaction**.
2. An in-process relay reads unpublished outbox rows in order and marks them
   published.
3. The relay hands events to an in-process WebSocket hub that fans them out to
   connected clients.

Explicitly **no** external broker — no Kafka, Redis, NATS, or RabbitMQ.

## Consequences

**Positive**

- No dual-write inconsistency: the event exists if and only if the state change
  committed.
- Zero extra infrastructure; everything runs in the monolith and one database.
- At-least-once delivery semantics are easy to reason about; the `published_at`
  marker and an index on unpublished rows keep the relay scan cheap.

**Negative / trade-offs**

- The hub is in-process, so horizontal scaling of WebSocket fan-out would
  require revisiting this decision (e.g. shared pub/sub) — acceptable for a
  single-process reference implementation.
- At-least-once delivery means consumers must tolerate duplicates (idempotent
  rendering on the client).

## Implementation

Migration `0001_init` creates `outbox_events` with a partial index on
unpublished rows. The transactions and alerts stores write their events in the
same database transaction as the state change. `internal/realtime` polls the
outbox (publish, then mark `published_at`), fans events out through an
in-process hub, and serves them at `/ws` per `contracts/asyncapi.yaml`.
`LISTEN/NOTIFY` is not used; the poll interval is configurable via
`OUTBOX_POLL_INTERVAL`.

## Alternatives considered

- **Direct broker (Kafka/NATS/Redis/RabbitMQ)** — rejected: out of scope and
  adds operational weight inconsistent with a local-first reference project.
- **Listen/Notify only** — rejected as the primary mechanism: `NOTIFY` is
  fire-and-forget and loses events if no listener is connected; the outbox
  provides durability. (`LISTEN/NOTIFY` may later complement the relay as a
  low-latency wake-up.)
