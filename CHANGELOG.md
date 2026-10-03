# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- A copyable AI-agent prompt and a complete local Docker sandbox walkthrough.

### Fixed

- When a CEL rule errors at runtime, the evaluation now fails secure to `ALRT`
  instead of silently degrading to NALT, the other rules still score, and the
  evaluation error is surfaced in the response `reasons`.
- Startup now fails loudly when the database rules/typologies loader errors
  (e.g. connection loss, query timeout, or a corrupt row). Previously the
  loaders swallowed the error and started with zero rules/typologies, silently
  classifying every transaction as `no_alert`. A legitimately empty database
  still starts cleanly for first-run onboarding.
- Sandbox checks now stop on Docker name or port conflicts and keep assurance
  ports bound to the local machine.
- `PUT /rules/{id}` disabling a rule referenced by a loaded typology now
  returns 409 (matching `DELETE /rules/{id}`) instead of silently dropping it
  from the engine and under-scoring the typology. Pass `?force=true` to opt in;
  the 200 response then carries a `warnings` array naming the affected
  typologies.

### Security

- `POST /evaluate` no longer lets caller `metadata` override engine-authoritative
  CEL variables. `metadata` keys that collide with authoritative Catalog names
  (`amount`, `currency`, `tx_type`, `debtor_id`, `creditor_id`, `tx`,
  `velocity_count`, `velocity_amount_sum`, `velocity_distinct_creditors`) are now
  dropped from the top-level activation merge instead of clobbering the real
  transaction value. Only the intended metadata-sourced variables (`old_balance`,
  `new_balance`) are honored. This closes a rule-bypass where, for example,
  `metadata:{"amount":1.0}` sent with a real 500000 transaction would make
  `amount > 200000.0` score 0 and silently pass a high-value fraud rule.
- Velocity controls now window on the server-controlled ingest time
  (`created_at`) instead of the client-supplied event timestamp, closing a
  bypass where any `POST /evaluate` caller could backdate `timestamp` to
  silence all velocity rules and the FATF typologies that weight them.
- Postgres startup now quotes the lib/pq connection string, so empty or
  whitespace-bearing credentials no longer fold neighbouring fields or fail
  DSN parsing.
- NATS event bus no longer crashes the process on NATS client errors that
  carry a nil subscription (reconnect-handshake read failures, transient
  server errors). The async error handler now guards the subscription before
  reading its subject, mirroring the library's own default handler.
- Typology `processMs` in `GET /evaluations/{id}` now reports each typology's
  own evaluation time instead of cumulative time since the batch started; the
  singular `EvaluateTypology` path now reports it consistently with the batch
  path.
- Per-tenant async worker now attributes evaluations and decision/alert events
  to the subscription tenant, not the `tenantId` field of the untrusted message
  payload; the global (testing/dev) path still recovers the tenant from the
  payload.

## [0.1.0] - 2026-07-07

First tagged release. Osprey evaluates transactions against CEL rules and returns
an `ALRT`/`NALT` decision, in a single deployable service.

### Added

- **Rule engine** — Google CEL expressions over a documented variable catalog,
  with detection (fast scoring) and compliance (FATF typology) evaluation modes.
- **Extensible variables** — `meta` (caller-asserted facts) and `enrichment`
  (externally computed signals) maps, plus velocity aggregates
  (`velocity_count`, `velocity_amount_sum`, `velocity_distinct_creditors`).
  Introspection via `GET /rules/variables`.
- **Rule CRUD** — create, read, update, and delete rules and typologies over the API.
- **Tiers** — Community (SQLite + in-memory cache + Go channels) and Pro
  (PostgreSQL + Redis + NATS), selected via `OSPREY_TIER`.
- **Multi-tenancy** — `X-Tenant-ID` scoping across cache keys, storage, and messaging.
- **FATF starter kit** — seedable starter rules and typologies (`make seed`).
- **Operability** — structured JSON logging, `GET /health` with build version,
  admin-token-protected mutation endpoints, Docker image and docker-compose stack.
- **`--version` flag** — print version/commit/build metadata and exit.
- **Docs** — architecture, quickstart, sandbox, rule/typology authoring, load
  testing, and a full OpenAPI 3 contract.

### Security

- Mutation endpoints require `OSPREY_ADMIN_TOKEN`; the server refuses to start
  without one.

[Unreleased]: https://github.com/opensource-finance/osprey/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/opensource-finance/osprey/releases/tag/v0.1.0
