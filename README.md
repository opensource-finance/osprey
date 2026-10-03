# Osprey

[![CI](https://github.com/opensource-finance/osprey/actions/workflows/sandbox-assurance.yml/badge.svg)](https://github.com/opensource-finance/osprey/actions/workflows/sandbox-assurance.yml)
[![Release](https://img.shields.io/github/v/release/opensource-finance/osprey?sort=semver)](https://github.com/opensource-finance/osprey/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/opensource-finance/osprey.svg)](https://pkg.go.dev/github.com/opensource-finance/osprey)
[![Go Report Card](https://goreportcard.com/badge/github.com/opensource-finance/osprey)](https://goreportcard.com/report/github.com/opensource-finance/osprey)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Open-source transaction monitoring in one deployable service.

Osprey evaluates transactions against CEL rules. It returns one of these decisions:

- `ALRT`: alert
- `NALT`: no alert

Use Osprey if you need a simple rules engine for fraud or compliance. Osprey does not need a large platform.

## Start Locally

Osprey needs Go 1.26+.

1. Get the code and start the server:

```bash
git clone https://github.com/opensource-finance/osprey.git
cd osprey

export OSPREY_ADMIN_TOKEN=local-admin-token
go run ./cmd/osprey
```

2. In a different terminal, evaluate a normal transaction:

```bash
curl -fsS -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -d @docs/examples/evaluate-normal.json
```

3. Add the sample rule:

```bash
curl -fsS -X POST http://localhost:8080/rules \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json
```

4. Send a transaction that causes an alert:

```bash
curl -fsS -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -d @docs/examples/evaluate-alert.json
```

## Docs

Start here:

- [Quickstart](docs/QUICKSTART.md)
- [Sandbox and API guide](docs/SANDBOX.md)
- [Rule and typology authoring](docs/RULE_TYPOLOGY_AUTHORING.md)
- [Starter kit rules](docs/STARTER_KIT.md)
- [Architecture](docs/ARCHITECTURE.md)
- [OpenAPI contract](docs/api/openapi.yaml)
- [Documentation style](docs/STYLE.md): run `make docs-lint` before a pull request that changes docs.

Operator references:

- [Sandbox assurance](docs/ASSURANCE.md)
- [Load testing](docs/LOAD_TESTING.md)

## Evaluation Modes

| Mode | Use When | Behavior |
|------|----------|----------|
| `detection` | You want weighted fraud rules. | Rules give a score and a decision. |
| `compliance` | You want rules in groups (typologies). | Load the typologies before you evaluate transactions. |

Detection mode is the default. To start Osprey in detection mode, run this command:

```bash
OSPREY_ADMIN_TOKEN=local-admin-token \
OSPREY_MODE=detection \
go run ./cmd/osprey
```

To start Osprey in compliance mode, run this command:

```bash
OSPREY_ADMIN_TOKEN=local-admin-token \
OSPREY_MODE=compliance \
go run ./cmd/osprey
```

If Osprey starts in compliance mode without typologies, these results occur:

- `POST /evaluate` returns `503`
- `GET /health` reports `status: "degraded"`
- `GET /ready` returns `503`

## Runtime Profiles

| Profile | Backing Services |
|---------|------------------|
| `community` | SQLite, in-memory cache, channel bus |
| `pro` | PostgreSQL, Redis, NATS |

The `community` profile is the default. It is the easiest profile for a local Osprey server.

## Configuration

**WARNING:** Do not share `OSPREY_ADMIN_TOKEN`. The token gives write access to all rules and typologies.

| Variable | Default | Notes |
|----------|---------|-------|
| `OSPREY_ADMIN_TOKEN` | required | Osprey does not start without it. It protects rule and typology writes. |
| `OSPREY_MODE` | `detection` | `detection` or `compliance`. |
| `OSPREY_TIER` | `community` | `community` or `pro`. |
| `OSPREY_PORT` | `8080` | HTTP port. |
| `OSPREY_DB_DRIVER` | `sqlite` | `sqlite` or `postgres`. |
| `OSPREY_SQLITE_PATH` | `./osprey.db` | SQLite path. |
| `OSPREY_CACHE_TYPE` | `memory` | `memory` or `redis`. |
| `OSPREY_BUS_TYPE` | `channel` | `channel` or `nats`. |
| `OSPREY_TENANTS` | unset | Optional. A comma-separated list of tenants for async workers. |
| `OSPREY_RATE_LIMIT_RPS` | `0` (off) | Requests per second for each tenant. `0` disables the rate limit. Set `0` for load tests. |
| `OSPREY_RATE_LIMIT_BURST` | `= RPS` | Burst size for each tenant. |

## API

Each tenant request must have this header:

```http
X-Tenant-ID: <tenant-id>
```

Each endpoint that changes data must also have one admin token header:

```http
Authorization: Bearer <token>
```

Or use this header:

```http
X-Osprey-Admin-Token: <token>
```

Core endpoints:

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/evaluate` | Evaluate a transaction. |
| `GET` | `/rules` | List the active rules. |
| `GET` | `/rules/{id}` | Get one active rule. |
| `POST` | `/rules` | Create or update a rule. |
| `PUT` | `/rules/{id}` | Update a rule. |
| `DELETE` | `/rules/{id}` | Delete a rule. Returns `409` if a typology uses the rule. |
| `POST` | `/rules/reload` | Load the rules again from storage. |
| `GET` | `/health` | Health status. |
| `GET` | `/ready` | Shows if Osprey is ready for traffic. |

Typology endpoints:

| Method | Endpoint |
|--------|----------|
| `GET` | `/typologies` |
| `POST` | `/typologies` |
| `PUT` | `/typologies/{id}` |
| `DELETE` | `/typologies/{id}` |
| `POST` | `/typologies/reload` |

## Starter Kit

To load the public starter rules (based on FATF guidance), run these commands:

```bash
export OSPREY_ADMIN_TOKEN=local-admin-token
./scripts/seed-starter-kit.sh
```

To load the rules and the typologies for compliance mode, run these commands:

```bash
export OSPREY_ADMIN_TOKEN=local-admin-token
OSPREY_MODE=compliance go run ./cmd/osprey &
./scripts/seed-starter-kit.sh --compliance
```

**CAUTION:** Examine the starter rules before you use them in a live workflow. The rules are examples and a start point. They do not replace your own risk policy.

## Development

To run the unit tests, the vet checks, and the integration tests, run these commands:

```bash
go test ./...
go vet ./...
./scripts/test-integration.sh
```

To run the full sandbox gate, run this command:

```bash
./scripts/assure-sandbox.sh
```

## License

Apache License 2.0
