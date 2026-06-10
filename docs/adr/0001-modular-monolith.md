# ADR 0001: Modular monolith over microservices

- Status: Accepted
- Date: 2026-06-10

## Context

FinWatch is a reference implementation that must be easy to read, run, and
extend. The domain (transaction monitoring, rule evaluation, alerting,
operator workflows) is cohesive and shares a single data model. Contributors
should be able to run the whole system locally with one command.

## Decision

Build FinWatch as a **modular monolith**: one deployable Go process whose
internal feature modules communicate through in-process Go interfaces. Module
boundaries are enforced by package structure (`internal/<module>`), not by
network calls.

## Consequences

**Positive**

- Simplest possible local setup; one binary, one process to reason about.
- Strong consistency via a single PostgreSQL database and local transactions.
- Refactoring across module boundaries is a compile-time concern, not a
  distributed-systems problem.
- Clear seams remain, so a module could be extracted later if genuinely needed.

**Negative / trade-offs**

- Independent scaling of a single module is not possible without extraction.
- Discipline is required to keep module boundaries clean inside one process.

**Mitigations**

- Keep modules behind explicit interfaces and avoid shared mutable state.
- Use the transactional outbox (ADR 0003) so the realtime path does not couple
  modules through shared in-memory channels in fragile ways.

## Alternatives considered

- **Microservices** — rejected as premature complexity for a reference
  implementation; introduces network failure modes, distributed transactions,
  and heavier local tooling with no offsetting benefit at this scale.
