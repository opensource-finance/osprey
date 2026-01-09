# Osprey

**Transaction monitoring that deploys in 60 seconds.**

*The osprey never misses.*

---

## What is Osprey?

Osprey is an open-source transaction monitoring engine built for fintechs, crypto platforms, e-commerce, and gaming companies who need fraud detection without enterprise complexity.

**Two evaluation modes to match your needs:**

| Mode | Description | Best For |
|------|-------------|----------|
| **Detection** (default) | Fast, weighted rule scoring | Fraud detection, startups, product teams |
| **Compliance** | FATF-aligned typology evaluation | Banks, regulated fintechs, compliance teams |

**From the founding engineers of [Tazama](https://github.com/tazama-lf) (Gates Foundation -> Linux Foundation).**

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

# Response
{
  "status": "NALT",
  "score": 0.12,
  "evaluationId": "eval-abc123"
}
```

## Starter Kit

Osprey includes pre-built rules and typologies based on public FATF guidance:

```bash
# Load FATF-aligned rules (Detection mode)
./scripts/seed-starter-kit.sh

# Load with typologies (Compliance mode)
./scripts/seed-starter-kit.sh --compliance
```

**Included rules:** Structuring detection, high-value transactions, account drain, velocity checks, same-party transfers, and more.

**Included typologies:** Account Takeover, Structuring (Smurfing), Mule Account, Rapid Movement of Funds, Cash Intensive Business.

See [docs/STARTER_KIT.md](docs/STARTER_KIT.md) for the complete list and customization options.

## Evaluation Modes

### Detection Mode (Default)

Fast fraud detection with weighted rule scoring. No typologies required.

```
Transaction -> Rules -> Weighted Score -> Alert/Pass
```

- Sub-5ms evaluation latency
- Simple weighted rule aggregation
- Start detecting fraud immediately
- Upgrade to Compliance mode when needed

```bash
# Detection mode is the default
./osprey

# Or explicitly
OSPREY_MODE=detection ./osprey
```

### Compliance Mode

FATF-aligned evaluation with typologies for regulated entities.

```
Transaction -> Rules -> Typologies -> FATF Patterns -> Alert/Pass
```

- Typologies required (Account Takeover, Structuring, Mule Account, etc.)
- Full audit trails for SAR filing
- Pattern-based detection aligned with FATF guidance

```bash
# Enable Compliance mode (requires typologies to be configured)
OSPREY_MODE=compliance ./osprey
```

## Architecture

```
                          OSPREY (Single Binary)
+------------------------------------------------------------------+
|  +---------+    +--------+    +-----------+    +--------+        |
|  | Ingest  | -> | Rules  | -> |Aggregation| -> |  TADP  | -> OUT |
|  |  (API)  |    | (CEL)  |    |           |    |(Decide)|        |
|  +---------+    +--------+    +-----------+    +--------+        |
|                                     |                             |
|                    +----------------+----------------+            |
|                    |                                 |            |
|              Detection Mode                 Compliance Mode       |
|               (default)                    (typologies)           |
+------------------------------------------------------------------+
|  SQLite (Community)  |  PostgreSQL + Redis + NATS (Pro)          |
+------------------------------------------------------------------+
```

## Tiers

| Feature | Community (Free) | Pro ($299/mo) | Enterprise |
|---------|-----------------|---------------|------------|
| Detection Mode | Yes | Yes | Yes |
| Compliance Mode | Yes | Yes | Yes |
| SQLite | Yes | Yes | Yes |
| PostgreSQL | - | Yes | Yes |
| NATS (horizontal scale) | - | Yes | Yes |
| Redis caching | - | Yes | Yes |
| Rules | 3 | Unlimited | Unlimited |
| Multi-node | - | - | Yes |
| SSO | - | - | Yes |
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
│   ├── rules/           # CEL-Go rule engine + typology engine
│   ├── tadp/            # Decision engine (Detection/Compliance modes)
│   └── api/             # Chi HTTP handlers
└── docs/                # Architecture documentation
```

## Development

```bash
# Clone
git clone https://github.com/opensource-finance/osprey.git
cd osprey

# Build
go build -o osprey ./cmd/osprey

# Run (Detection mode)
./osprey

# Run (Compliance mode)
OSPREY_MODE=compliance ./osprey

# Test
go test ./...
```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `OSPREY_MODE` | `detection` | Evaluation mode: `detection` or `compliance` |
| `OSPREY_TIER` | `community` | Tier: `community`, `pro`, `enterprise` |
| `OSPREY_DEBUG` | `false` | Enable debug logging |
| `OSPREY_PORT` | `8080` | HTTP server port |
| `OSPREY_DB_DRIVER` | `sqlite` | Database: `sqlite`, `postgres` |
| `OSPREY_CACHE_TYPE` | `memory` | Cache: `memory`, `redis` |
| `OSPREY_BUS_TYPE` | `channel` | Event bus: `channel`, `nats` |

## API Endpoints

### Core (Both Modes)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/evaluate` | Evaluate a transaction |
| GET | `/rules` | List all rules |
| POST | `/rules` | Create a new rule |
| POST | `/rules/reload` | Hot-reload rules |
| GET | `/health` | Health check (includes mode) |

### Health Check Response

```bash
curl http://localhost:8080/health
```

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "mode": "detection"
}
```

The `mode` field confirms which evaluation mode the server is running in.

### Compliance Mode Only

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/typologies` | List all typologies |
| POST | `/typologies` | Create a new typology |
| PUT | `/typologies/{id}` | Update a typology |
| DELETE | `/typologies/{id}` | Delete a typology |
| POST | `/typologies/reload` | Hot-reload typologies |

## License

Apache License 2.0

## Links

- **Website:** [opensource.finance](https://opensource.finance)
- **Documentation:** [docs.opensource.finance](https://docs.opensource.finance)
- **GitHub:** [github.com/opensource-finance/osprey](https://github.com/opensource-finance/osprey)

---

*Banks have Tazama. Everyone else has Osprey.*
