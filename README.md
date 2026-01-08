# Osprey

**Transaction monitoring that deploys in 60 seconds.**

*The osprey never misses.*

---

## What is Osprey?

Osprey is an open-source transaction monitoring engine built for fintechs, crypto platforms, e-commerce, and gaming companies who need fraud detection without enterprise complexity.

**From the founding engineers of [Tazama](https://github.com/tazama-lf) (Gates Foundation → Linux Foundation).**

## Why Osprey?

| Enterprise Solutions | Osprey |
|---------------------|--------|
| 7+ microservices | Single binary |
| Kubernetes required | Run anywhere |
| Weeks to deploy | 60 seconds |
| DevOps team needed | Any developer |
| $50K+/year | Free to start |

## Quick Start

```bash
# Download
curl -fsSL https://osprey.opensource.finance/install.sh | sh

# Run
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

# Response
{
  "status": "PASS",
  "score": 0.12,
  "evaluationId": "eval-abc123"
}
```

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                 OSPREY (Single Binary)              │
├─────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌───────┐  │
│  │ Ingest  │→ │  Rules  │→ │Typology │→ │ TADP  │  │
│  │  (API)  │  │ (CEL)   │  │ (Score) │  │(Decide)│ │
│  └─────────┘  └─────────┘  └─────────┘  └───────┘  │
├─────────────────────────────────────────────────────┤
│  SQLite (Community) │ PostgreSQL + NATS (Pro)      │
└─────────────────────────────────────────────────────┘
```

## Tiers

| Feature | Community (Free) | Pro ($299/mo) | Enterprise |
|---------|-----------------|---------------|------------|
| Full engine | ✅ | ✅ | ✅ |
| SQLite | ✅ | ✅ | ✅ |
| PostgreSQL | - | ✅ | ✅ |
| NATS (horizontal scale) | - | ✅ | ✅ |
| Redis caching | - | ✅ | ✅ |
| Rules | 3 | Unlimited | Unlimited |
| Multi-node | - | - | ✅ |
| SSO | - | - | ✅ |
| Support | Community | Email | Dedicated |

## Tech Stack

- **Language:** Go 1.22+
- **Rule Engine:** Google CEL-Go
- **Web Framework:** Chi (stdlib-compatible)
- **Database:** SQLite (default) / PostgreSQL (Pro)
- **Caching:** In-memory LRU / Redis (Pro)
- **Messaging:** Go channels / NATS (Pro)
- **Observability:** slog + OpenTelemetry

## Project Structure

```
osprey/
├── cmd/osprey/          # Application entrypoint
├── internal/
│   ├── domain/          # Core interfaces and types
│   ├── repository/      # SQLite + PostgreSQL implementations
│   ├── cache/           # LRU + Redis implementations
│   ├── bus/             # Channels + NATS implementations
│   ├── rules/           # CEL-Go rule engine
│   ├── tadp/            # Decision engine
│   └── api/             # Chi HTTP handlers
└── pkg/                 # Public utilities
```

## Development

```bash
# Clone
git clone https://github.com/opensource-finance/osprey.git
cd osprey

# Build
go build -o osprey ./cmd/osprey

# Run
./osprey

# Test
go test ./...
```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `OSPREY_TIER` | `community` | Tier: `community`, `pro`, `enterprise` |
| `OSPREY_PORT` | `8080` | HTTP server port |
| `OSPREY_DB_DRIVER` | `sqlite` | Database: `sqlite`, `postgres` |
| `OSPREY_CACHE_TYPE` | `memory` | Cache: `memory`, `redis` |
| `OSPREY_BUS_TYPE` | `channel` | Event bus: `channel`, `nats` |

## License

Apache License 2.0

## Links

- **Website:** [opensource.finance](https://opensource.finance)
- **Documentation:** [docs.opensource.finance](https://docs.opensource.finance)
- **GitHub:** [github.com/opensource-finance/osprey](https://github.com/opensource-finance/osprey)

---

*Banks have Tazama. Everyone else has Osprey.*
