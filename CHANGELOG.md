# Changelog

This file records all notable changes to this project. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). This project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `examples/`: two small projects that use Osprey, with walkthroughs and real
  output. One finds money-mule activity in detection mode. One finds structuring
  and sanctions matches in compliance mode. `make examples` runs both and fails
  if a decision changes.
- `OSPREY_VELOCITY_CLOCK=event` makes velocity windows use the `timestamp` field of each transaction.
  Use it to replay historical data. The default, `received`, uses the time when Osprey received the transaction.
- `OSPREY_ALERT_THRESHOLD` sets the detection-mode alert threshold. Before this change, `0.7` was fixed in the code.
- `OSPREY_SCORING` selects the detection-mode scoring: `sum` (the new default) or `average`.
- The CEL variables `velocity_inbound_count` and `velocity_inbound_amount_sum`.
  They count the payments that the debtor received in the velocity window.
- The starter-kit rules `pass-through-001` and `fan-out-001`.
- `cmd/qualitybench` measures precision and recall of a rule set on the labelled PaySim dataset.
  With `-velocity`, it replays the rows in order with velocity windows on PaySim time.
- `cmd/benchmark` sends the PaySim time of each row as `timestamp`.

### Changed

- **Breaking:** detection mode now adds `score x weight` for each rule, with a maximum of `1.0`.
  Before this change, the score was the weighted average over all active rules.
  Each new rule lowered the score of every transaction that it did not match.
  To keep the old behavior, set `OSPREY_SCORING=average`.
  Rule sets that were tuned for the average can alert much more with `sum`, so examine your weights.
- The starter kit (`fatf-rules.json`, `fatf-typologies.json`, `paysim-rules.json`) is now version 2.0.0.
  No rule uses `.fail` for a weak signal. Account drain caused an alert alone before, which flagged 24% of PaySim traffic.
  On all of PaySim, the PaySim rules now alert on 0.13% of transactions with 97.6% recall.
  The [starter kit guide](docs/STARTER_KIT.md) gives the measured numbers and their limits.
- `seed-paysim.sh` and `paysim-rules.json` no longer say "100% precision" without context.
  That claim was false for the v1 rules (0.5% precision).

### Fixed

- Velocity lookups no longer read every transaction of the tenant in the window.
  The query used `debtor_id = ? OR creditor_id = ?`, and SQLite searched the `(tenant_id, created_at)` index.
  Thus, each evaluation got slower as traffic grew.
  Now the query searches new `(tenant_id, debtor_id, ...)` and `(tenant_id, creditor_id, ...)` indexes.
  With the HTTP benchmark on 20,000 PaySim rows, throughput went from 84 to 838 transactions per second.
  Osprey creates the new indexes at startup.
- `velocity_distinct_creditors` now counts only the payees that the debtor sent money to.
  Before this change, it also counted payments that the debtor received, so an account that received money counted itself as a payee.
- Osprey rounds detection and typology scores to 9 decimal places before it compares them with the threshold.
  Before this change, a sum could miss its threshold by the last binary digit.
  For example, weights `0.1` and `0.7` add up to `0.7999999999999999`, which is less than a threshold of `0.8`.

- SQLite now opens the correct file and uses WAL mode when `OSPREY_SQLITE_PATH` contains `?`, `#`, or `%`.
  Before this change, Osprey put the path into the SQLite connection URI with no encoding.
  A `?` started the query part of the URI, so SQLite opened a file at the part of the path before the `?`.
  The first pragma was also lost, so `journal_mode` was `delete` instead of `wal`.
  Now Osprey encodes these characters, and the database file has the full path.
- A compliance evaluation no longer uses detection scoring when the typology set becomes empty during the request.
  Before this change, `POST /evaluate` and the async worker checked for loaded typologies at the start.
  If an operator deleted or disabled the last typology after that check, the processor used the detection aggregate score.
  A transaction that a typology flags could get `NALT` with `200 OK`, and the record showed only a `detection-summary` result.

  Now the processor fails closed. The evaluation gets `ALRT` and one typology result with the ID `compliance-no-typologies`.
  Osprey logs a warning. The transaction and the evaluation are both stored, so a retry is not necessary.
- PostgreSQL deployments now keep the full value of transaction amounts and scores.
  Before this change, the migration kept the SQLite `REAL` column type on PostgreSQL.
  PostgreSQL `REAL` is single precision (`float4`, about 7 significant digits).
  The `amount`, `weight`, `score`, and `alert_threshold` columns rounded large values with no error.
  For example, an amount of `1234567.89` was stored and returned as `1234567.9`.
  The rounded amount also went into `velocity_amount_sum` for velocity rules.

  The migration now uses `DOUBLE PRECISION` (`float8`) on PostgreSQL, the same precision as SQLite.
  At startup, the migration also changes existing `real` columns to `double precision`.
  This step keeps the value that the client saw before, so `0.1` stays `0.1`.
  The upgrade cannot restore digits that `float4` already removed.
- PostgreSQL deployments now start on a fresh database. Before this change, the
  `transactions` migration used the SQLite `BLOB` column type, which PostgreSQL
  does not have. The `CREATE TABLE` statement failed with `type "blob" does not
  exist` and the process exited. The migration now rewrites `BLOB` to `BYTEA`
  for PostgreSQL while SQLite keeps the original `BLOB` type.
- A rule whose score equals the `UpperLimit` of the last band now matches that
  band instead of falling through to the default `.pass`.
  Before this change, `matchBand` always continued to a next band that may not
  exist. A closed last band returned `no matching band` and the outcome
  became `.pass`.
  When the last band's outcome was `.fail`, the TADP aggregator never set
  `HasCriticalFailure`, and a transaction below `AlertThreshold` got `NALT`
  instead of `ALRT`. A fraudulent transaction went undetected.
- `PUT /typologies/{id}` now requires the `enabled` field.
  Before this change, a request body that omitted `enabled` persisted `enabled: false`.
  The handler returned `200 OK` and the reload dropped the typology from the active engine.
  In compliance mode this changed the final decision, or returned `503` on every later `POST /evaluate` when it was the last enabled typology.
  The handler now returns `400 enabled is required`, the same guard `PUT /rules/{id}` already has.
  An explicit `enabled: false` is still an accepted, intentional disable.

### Security

- Account-drain rules now fire when `metadata.new_balance` is `null`.
  Before this change, the rule engine copied `old_balance` and `new_balance` from `metadata` with no type check.
  A `null` or a string replaced the `0.0` default, and the CEL `==` operator returned `false` for the different types.
  For example, `{"old_balance": 15000, "new_balance": null}` made `old_balance > 0.0 && new_balance == 0.0` false, and the drain got `.pass`.

  Now a `null` balance gives `0.0`, the same as a missing balance.
  A balance that is not a number or `null` makes `POST /evaluate` return `400`.
  The async worker logs an error and publishes no decision.
- A very large `velocityWindow` no longer disables velocity rules.
  Before this change, the async worker passed any positive `velocityWindow` to the velocity service.
  A value above about `9223372036` seconds overflowed the window duration and put the window start in the future.
  All velocity metrics read `0`, and a high-velocity transaction got `NALT` instead of `ALRT` with no error.
  The velocity service now clamps the window to 30 days (`2592000` seconds), and the worker clamps the value and logs a warning.
- The async worker now defaults a non-positive `velocityWindow` in a transaction message to `3600` (one hour).
  Before this change, the worker defaulted only a `0` value and let a negative value pass to the rule engine.
  The engine skips velocity lookups when `VelocityWindow <= 0`, so a negative value disabled all velocity rules for that transaction.
  A high-velocity transaction got `NALT` instead of `ALRT` with no error or warning.

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
