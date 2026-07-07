# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
