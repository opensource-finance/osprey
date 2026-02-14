# Osprey

**Transaction monitoring that deploys in 60 seconds.**

*The osprey never misses.*

---

## What is Osprey?

Osprey is an open-source transaction monitoring engine built for fintechs, crypto platforms, e-commerce, and gaming companies who need fraud detection without platform sprawl.

**Two evaluation modes:**

| Mode | Description | Best For |
|------|-------------|----------|
| **Detection** (default) | Fast, weighted rule scoring | Fraud detection, startups, product teams |
| **Compliance** | FATF-aligned typology evaluation | Regulated fintechs and compliance teams |

**From the founding engineers of [Tazama](https://github.com/tazama-lf) (Gates Foundation -> Linux Foundation).**

## Why Osprey?

| Traditional Platforms | Osprey |
|----------------------|--------|
| 7+ microservices | Single binary |
| Kubernetes-heavy setup | Run anywhere |
| Weeks to deploy | 60 seconds |
| Dedicated DevOps needed | Any developer can start |
| Expensive licensing + ops | Open source core |

## Quick Start

```bash
# Download
curl -fsSL https://osprey.opensource.finance/install.sh | sh

# Run (Detection mode by default)
./osprey

# Evaluate a transaction
curl -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -d '{
    "type": "transfer",
    "debtor": {"id": "user-123", "accountId": "acc-456"},
    "creditor": {"id": "user-789", "accountId": "acc-012"},
    "amount": {"value": 1000, "currency": "USD"}
  }'
```

## Starter Kit

Osprey includes pre-built rules and typologies based on public FATF guidance:

```bash
# Load FATF-aligned rules (Detection mode)
./scripts/seed-starter-kit.sh

# Load rules + typologies (Compliance mode)
./scripts/seed-starter-kit.sh --compliance
```

See [docs/STARTER_KIT.md](docs/STARTER_KIT.md) for complete rule/typology lists.

## Evaluation Modes

### Detection Mode (Default)

Fast fraud detection with weighted rule scoring.

```
Transaction -> Rules -> Weighted Score -> Alert/Pass
```

- No typologies required
- Low-latency evaluation
- Good default for product-led fraud prevention

```bash
./osprey
# or
OSPREY_MODE=detection ./osprey
```

### Compliance Mode

FATF-aligned evaluation with typologies.

```
Transaction -> Rules -> Typologies -> FATF Patterns -> Alert/Pass
```

- Typologies are required for evaluation
- If Compliance mode is enabled with no typologies loaded:
- `/evaluate` returns `503 Service Unavailable`
- `/health` reports `status: "degraded"`
- `/ready` returns `503` with `{"ready":"false"}`

```bash
OSPREY_MODE=compliance ./osprey
```

## Runtime Profiles

Osprey supports two runtime profiles:

| Profile | Infrastructure |
|---------|----------------|
| **Community** (default) | SQLite + in-memory cache + channel bus |
| **Pro profile** (`OSPREY_TIER=pro`) | PostgreSQL + Redis + NATS |

`OSPREY_TIER=enterprise` is currently treated as unsupported in the open-source build and falls back to community defaults with a warning.

## Tech Stack

- **Language:** Go 1.25+
- **Rule Engine:** Google CEL-Go
- **Web Framework:** Chi
- **Database:** SQLite (default) / PostgreSQL (pro profile)
- **Caching:** In-memory LRU / Redis (pro profile)
- **Messaging:** Go channels / NATS (pro profile)
- **Observability:** slog + OpenTelemetry

## Development

```bash
# Clone
git clone https://github.com/opensource-finance/osprey.git
cd osprey

# Build
go build -o osprey ./cmd/osprey

# Unit + package tests (default)
go test ./...

# Integration tests (explicit)
./scripts/test-integration.sh
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OSPREY_MODE` | `detection` | Evaluation mode: `detection` or `compliance` |
| `OSPREY_TIER` | `community` | Runtime profile: `community` or `pro` |
| `OSPREY_DEBUG` | `false` | Enable debug logging |
| `OSPREY_PORT` | `8080` | HTTP server port |
| `OSPREY_DB_DRIVER` | `sqlite` | Database: `sqlite`, `postgres` |
| `OSPREY_CACHE_TYPE` | `memory` | Cache: `memory`, `redis` |
| `OSPREY_BUS_TYPE` | `channel` | Event bus: `channel`, `nats` |

## API Endpoints

### Core

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/evaluate` | Evaluate a transaction |
| GET | `/rules` | List loaded rules |
| POST | `/rules` | Create a rule (stored, requires reload to apply) |
| POST | `/rules/reload` | Reload rules from database |
| GET | `/health` | Health status |
| GET | `/ready` | Readiness status |

### Typology Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/typologies` | List loaded typologies |
| POST | `/typologies` | Create a typology |
| PUT | `/typologies/{id}` | Update a typology |
| DELETE | `/typologies/{id}` | Delete a typology |
| POST | `/typologies/reload` | Reload typologies from database |

## License

Apache License 2.0

## Links

- **Website:** [opensource.finance](https://opensource.finance)
- **Documentation:** [docs.opensource.finance](https://docs.opensource.finance)
- **GitHub:** [github.com/opensource-finance/osprey](https://github.com/opensource-finance/osprey)

---

*Banks have Tazama. Everyone else has Osprey.*
