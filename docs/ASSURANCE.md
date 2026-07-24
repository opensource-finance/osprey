# Sandbox Assurance

Use these checks before publishing or updating a shared Osprey sandbox.

## Required Tools

```text
curl, Go 1.26+, jq, Ruby, Docker
```

Run the commands below from the repository root. The local gate uses two host
ports, and both must be unused:

| Variable | Default | Purpose |
|----------|---------|---------|
| `OSPREY_TEST_PORT` | `18080` | Local HTTP integration server. |
| `DOCKER_PORT` | `18081` | Docker sandbox verification. |

Override either value when it conflicts with another local service. The
integration runner fails immediately if its target health endpoint already
responds, preventing it from seeding or testing the wrong Osprey process.

## Local Gate

```bash
VERSION=sandbox-YYYYMMDD \
OSPREY_TEST_PORT=18080 \
DOCKER_PORT=18081 \
./scripts/assure-sandbox.sh
```

This validates:

- JSON examples
- OpenAPI syntax, refs, examples, and route coverage
- Markdown links and PNG assets
- shell script syntax
- sandbox-script input and occupied-port regression tests
- Go tests, `go vet`, and race tests
- HTTP integration tests
- Docker build metadata, healthcheck, and API smoke behavior

## Public URL Gate

`verify-sandbox.sh` changes its target. It creates or replaces the global rules
`sandbox-verification-same-party` and `sandbox-verification-typology`, submits
transactions, and checks a second tenant. Run it only against a disposable
sandbox or with the deployment operator's approval.

```bash
OSPREY_URL=https://your-osprey-host.example \
TENANT_ID=demo \
OSPREY_ADMIN_TOKEN=replace-with-admin-token \
EXPECTED_STATUS=healthy \
EXPECTED_MODE=detection \
EXPECTED_VERSION=sandbox-YYYYMMDD \
./scripts/verify-sandbox.sh
```

This verifies:

- `/health` and `/ready`
- expected status, mode, and version when provided
- admin-token protection
- rule creation and immediate activation
- typology creation and immediate activation
- `NALT` response for a normal transaction
- `ALRT` response for a same-party transaction
- evaluation and transaction retrieval
- duplicate transaction conflict handling
- tenant isolation

`OSPREY_URL` and `OSPREY_ADMIN_TOKEN` are required. There is no implicit public
sandbox target.

Do not publish a sandbox URL until both gates pass.
