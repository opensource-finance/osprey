# Osprey Integration Tests

These tests are end-to-end tests. They need an Osprey server that runs and has seeded rules.

## Default Behavior

The `integration` build tag controls the integration tests. This command does **not** run them:

```bash
go test ./...
```

## Recommended Run

Use the dedicated runner script. The script does these steps:

1. It starts Osprey from `/tmp`.
2. It seeds minimal test rules.
3. It runs the test suite.

```bash
./scripts/test-integration.sh
```

## Manual Run

To run the tests manually, do these steps:

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

`./scripts/seed-rules.sh` creates the minimal rules that this suite expects:

- `high-value-001`
- `same-account-001`
- `amount-check-001`

These tests use only this minimal rule set.

**NOTE:** More rule packs in the same SQLite database can change the scores and the outcomes.

## Environment Variables

- `OSPREY_TEST_URL` (default: `http://localhost:8080`)

## Troubleshooting

### `connection refused`

The server does not run, or the server is not healthy.

### Unexpected scores/outcomes

Your database possibly has extra seeded rules or typologies. To get a clean state that you can reproduce, use `./scripts/test-integration.sh`.
