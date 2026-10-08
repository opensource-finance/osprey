# Changelog

This file records all notable changes to this project. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). This project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- `PUT /typologies/{id}` now requires the `enabled` field.
  Before this change, a request body that omitted `enabled` persisted `enabled: false`.
  The handler returned `200 OK` and the reload dropped the typology from the active engine.
  In compliance mode this changed the final decision, or returned `503` on every later `POST /evaluate` when it was the last enabled typology.
  The handler now returns `400 enabled is required`, the same guard `PUT /rules/{id}` already has.
  An explicit `enabled: false` is still an accepted, intentional disable.

## [0.1.1] - 2026-10-03

### Added

- A prompt for AI agents that you can copy. A full walkthrough for the local Docker sandbox.

### Fixed

- If a CEL rule causes an error at runtime, the evaluation fails secure to `ALRT`.
  Before this change, the result was `NALT` and the error was not visible.
  The other rules still score. The response `reasons` field shows the evaluation error.
- If the database loader for rules or typologies returns an error, startup now stops with a clear error.
  Examples of errors are a connection loss, a query timeout, or a corrupt row.
  Before this change, the loaders ignored the error and started with zero rules or typologies.
  The result was that every transaction got the `no_alert` decision.
  An empty database still starts correctly for the first run.
- Sandbox checks now stop if a Docker name or a port is already in use.
  The assurance ports now bind only to the local machine.
- If `PUT /rules/{id}` disables a rule that a loaded typology uses, the request now returns 409.
  This is the same as `DELETE /rules/{id}`.
  Before this change, the engine removed the rule without a message, and the typology score was too low.
  To disable the rule anyway, send `?force=true`.
  The 200 response then has a `warnings` array that names the typologies that the change affects.

### Security

- `POST /evaluate` no longer lets the caller `metadata` replace the CEL variables that the engine controls.
  These authoritative Catalog names are `amount`, `currency`, `tx_type`, `debtor_id`, `creditor_id`, `tx`,
  `velocity_count`, `velocity_amount_sum`, and `velocity_distinct_creditors`.
  The engine now drops `metadata` keys with these names from the top-level activation merge, so the real transaction values stay.
  The engine accepts only the two intended metadata variables, `old_balance` and `new_balance`.
  This change closes a rule bypass.
  For example, `metadata:{"amount":1.0}` on a real 500000 transaction gave `amount > 200000.0` a score of 0.
- Velocity controls now use the ingest time that the server sets (`created_at`) for the window.
  Before this change, they used the event timestamp from the client.
  Any `POST /evaluate` caller could set an old `timestamp` and stop all velocity rules.
  This also stopped the FATF typologies that give weight to velocity rules.
- Postgres startup now quotes the lib/pq connection string.
  Empty credentials or credentials with spaces no longer merge with the fields next to them.
  They also no longer cause DSN parse errors.
- The NATS event bus no longer stops the process on a NATS client error with a nil subscription.
  Reconnect handshake read failures and temporary server errors can cause this condition.
  The async error handler now checks the subscription before it reads the subject.
  The default handler of the library does the same check.
- Typology `processMs` in `GET /evaluations/{id}` now shows the evaluation time of each typology.
  Before this change, it showed the total time from the start of the batch.
  The single `EvaluateTypology` path now reports this value in the same way as the batch path.
- The async worker for each tenant now assigns evaluations and decision or alert events to the subscription tenant.
  It does not use the `tenantId` field from the untrusted message payload.
  The global path for tests and development still gets the tenant from the payload.

## [0.1.0] - 2026-07-07

This is the first tagged release. Osprey evaluates transactions against CEL rules.
It returns an `ALRT` or `NALT` decision from a single service that you can deploy.

### Added

- **Rule engine:** Google CEL expressions over a documented variable catalog.
  Two evaluation modes are available: detection (fast scoring) and compliance (FATF typology).
- **Extensible variables:** the `meta` map holds facts that the caller asserts.
  The `enrichment` map holds signals that an external system calculates.
  Velocity aggregates are also available: `velocity_count`, `velocity_amount_sum`, and `velocity_distinct_creditors`.
  `GET /rules/variables` lists the variables.
- **Rule CRUD:** the API lets you create, read, update, and delete rules and typologies.
- **Tiers:** Community (SQLite, in-memory cache, and Go channels) and Pro
  (PostgreSQL, Redis, and NATS). Use `OSPREY_TIER` to select the tier.
- **Multi-tenancy:** `X-Tenant-ID` sets the scope for cache keys, storage, and messages.
- **FATF starter kit:** starter rules and typologies that you can seed (`make seed`).
- **Operability:** structured JSON logs and `GET /health` with the build version.
  An admin token protects the mutation endpoints.
  The release includes a Docker image and a docker-compose stack.
- **`--version` flag:** prints the version, commit, and build metadata, then exits.
- **Docs:** architecture, quickstart, sandbox, rule and typology authoring, load
  testing, and a full OpenAPI 3 contract.

### Security

- Mutation endpoints require `OSPREY_ADMIN_TOKEN`.
  The server does not start without it.

[Unreleased]: https://github.com/opensource-finance/osprey/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/opensource-finance/osprey/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/opensource-finance/osprey/releases/tag/v0.1.0
