# System context

FinWatch is an open-source reference implementation for near-real-time
transaction monitoring and operational alert workflows using **synthetic data**.
It is not a certified fraud, AML, sanctions, or compliance product.

## Purpose

Demonstrate a clean, idiomatic architecture for ingesting synthetic
transactions, evaluating operational rules, raising alerts, and surfacing them
to operators in near real time — with a contract-first, modular-monolith design.

## Actors and external systems

| Actor / system        | Role                                                                 |
| --------------------- | -------------------------------------------------------------------- |
| **Operator** (human)  | Views dashboards, triages alerts, manages operational workflows via the web app. |
| **Web application**   | React SPA; calls the REST API and (later) subscribes to the WebSocket stream. |
| **FinWatch API**      | Go modular monolith; owns business logic, persistence, and realtime fan-out. |
| **PostgreSQL**        | System of record and the transactional outbox for realtime events.   |
| **Synthetic source** (future) | A generator/seed that produces synthetic transactions. No real data, ever. |

There are no external brokers, third-party data feeds, or production banking
integrations. Everything runs locally via Docker Compose.

## Context diagram

```
            +-----------+        HTTPS / WSS         +------------------+
  Operator  |   Web     |  ----------------------->  |   FinWatch API   |
   (human)  |  (React)  |  <-----------------------  |  (Go monolith)   |
            +-----------+    REST + event stream     +---------+--------+
                                                               |
                                                       SQL + outbox
                                                               |
                                                      +--------v--------+
                                                      |   PostgreSQL    |
                                                      +-----------------+
```

## Boundaries and guarantees

- **Synthetic-only data.** No real personal or financial data enters the system.
- **Contract-first.** REST is described by `contracts/openapi.yaml`; events by
  `contracts/asyncapi.yaml`.
- **Single deployable.** One API process; modules communicate in-process.
- **Money & time.** Integer minor units for money; UTC internally, RFC 3339 at
  boundaries.

## Out of scope (this bootstrap)

Transaction ingestion, rule evaluation, alerting, authentication/RBAC, and live
WebSocket streaming. These are tracked as follow-up issues.
