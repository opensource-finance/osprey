# Osprey Integration Tests

These are end-to-end tests that require a running Osprey server with seeded rules.

## Default Behavior

Integration tests are gated behind the `integration` build tag, so they are **not** run by default:

```bash
go test ./...
```

## Recommended Run

Use the dedicated runner, which starts Osprey from `/tmp`, seeds minimal test rules, and executes the suite:

```bash
./scripts/test-integration.sh
```

## Manual Run

If you want to run manually:

```bash
# 1) Start server
cd /path/to/osprey
go run ./cmd/osprey

# 2) Seed minimal integration rules
./scripts/seed-rules.sh

# 3) Run integration tests with build tag
go test -tags=integration -v ./tests/integration/...
```

## Seeded Rule Set

`./scripts/seed-rules.sh` creates the minimal rules expected by this suite:

- `high-value-001`
- `same-account-001`
- `amount-check-001`

These tests assume this minimal rule set. Loading additional rule packs into the same SQLite database can change scores and outcomes.

## Environment Variables

- `OSPREY_TEST_URL` (default: `http://localhost:8080`)

## Troubleshooting

### `connection refused`
Server is not running or not healthy.

### Unexpected scores/outcomes
You are likely using a database with extra seeded rules/typologies. Use `./scripts/test-integration.sh` for clean, reproducible state.
