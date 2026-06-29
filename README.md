# Osprey

Open-source transaction monitoring in one deployable service.

Osprey evaluates transactions against CEL rules and returns a decision:

- `ALRT`: alert
- `NALT`: no alert

Use it when you need a simple fraud or compliance rules engine without a large platform footprint.

## Start Locally

Requirements: Go 1.25+.

```bash
git clone https://github.com/opensource-finance/osprey.git
cd osprey

export OSPREY_ADMIN_TOKEN=local-admin-token
go run ./cmd/osprey
```

In another terminal:

```bash
curl -fsS -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -d @docs/examples/evaluate-normal.json
```

Add the sample rule:

```bash
curl -fsS -X POST http://localhost:8080/rules \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json
```

Then trigger an alert:

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

Operator references:

- [Sandbox assurance](docs/ASSURANCE.md)
- [Load testing](docs/LOAD_TESTING.md)

## Evaluation Modes

| Mode | Use When | Behavior |
|------|----------|----------|
| `detection` | You want weighted fraud rules. | Rules produce a score and decision. |
| `compliance` | You want rules grouped into typologies. | Typologies must be loaded before evaluation is ready. |

Detection mode is the default:

```bash
OSPREY_ADMIN_TOKEN=local-admin-token \
OSPREY_MODE=detection \
go run ./cmd/osprey
```

Compliance mode:

```bash
OSPREY_ADMIN_TOKEN=local-admin-token \
OSPREY_MODE=compliance \
go run ./cmd/osprey
```

If compliance mode starts without typologies:

- `POST /evaluate` returns `503`
- `GET /health` reports `status: "degraded"`
- `GET /ready` returns `503`

## Runtime Profiles

| Profile | Backing Services |
|---------|------------------|
| `community` | SQLite, in-memory cache, channel bus |
| `pro` | PostgreSQL, Redis, NATS |

Community is the default and is the easiest way to run Osprey locally.

## Configuration

| Variable | Default | Notes |
|----------|---------|-------|
| `OSPREY_ADMIN_TOKEN` | required | Required to start. Protects rule and typology writes. |
| `OSPREY_MODE` | `detection` | `detection` or `compliance`. |
| `OSPREY_TIER` | `community` | `community` or `pro`. |
| `OSPREY_PORT` | `8080` | HTTP port. |
| `OSPREY_DB_DRIVER` | `sqlite` | `sqlite` or `postgres`. |
| `OSPREY_SQLITE_PATH` | `./osprey.db` | SQLite path. |
| `OSPREY_CACHE_TYPE` | `memory` | `memory` or `redis`. |
| `OSPREY_BUS_TYPE` | `channel` | `channel` or `nats`. |
| `OSPREY_TENANTS` | unset | Optional comma-separated tenants for async workers. |
| `OSPREY_RATE_LIMIT_RPS` | `0` (off) | Per-tenant requests/second. `0` disables rate limiting. Leave off for load testing. |
| `OSPREY_RATE_LIMIT_BURST` | `= RPS` | Per-tenant burst size. |

## API

Every tenant-scoped request needs:

```http
X-Tenant-ID: <tenant-id>
```

Mutation endpoints also need one admin-token header:

```http
Authorization: Bearer <token>
```

or:

```http
X-Osprey-Admin-Token: <token>
```

Core endpoints:

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/evaluate` | Evaluate a transaction. |
| `GET` | `/rules` | List active rules. |
| `GET` | `/rules/{id}` | Fetch one active rule. |
| `POST` | `/rules` | Create or update a rule. |
| `PUT` | `/rules/{id}` | Update a rule. |
| `DELETE` | `/rules/{id}` | Delete a rule (`409` if referenced by a typology). |
| `POST` | `/rules/reload` | Reload rules from storage. |
| `GET` | `/health` | Health status. |
| `GET` | `/ready` | Traffic readiness. |

Typology endpoints:

| Method | Endpoint |
|--------|----------|
| `GET` | `/typologies` |
| `POST` | `/typologies` |
| `PUT` | `/typologies/{id}` |
| `DELETE` | `/typologies/{id}` |
| `POST` | `/typologies/reload` |

## Starter Kit

Load public FATF-inspired starter rules:

```bash
export OSPREY_ADMIN_TOKEN=local-admin-token
./scripts/seed-starter-kit.sh
```

Load rules and typologies for compliance mode:

```bash
export OSPREY_ADMIN_TOKEN=local-admin-token
OSPREY_MODE=compliance go run ./cmd/osprey &
./scripts/seed-starter-kit.sh --compliance
```

Review the rules before using them in a live workflow. They are examples and starting points, not a substitute for your own risk policy.

## Development

```bash
go test ./...
go vet ./...
./scripts/test-integration.sh
```

Full sandbox gate:

```bash
./scripts/assure-sandbox.sh
```

## License

Apache License 2.0
