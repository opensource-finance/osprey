# Sandbox Assurance

Do these checks before you publish or update a shared Osprey sandbox.

## Required Tools

```text
curl, Go 1.26+, jq, Ruby, Docker
```

Run the commands in this document from the repository root.
The local gate uses two host ports.
Make sure that no other process uses these ports.

| Variable | Default | Purpose |
|----------|---------|---------|
| `OSPREY_TEST_PORT` | `18080` | Local HTTP integration server. |
| `DOCKER_PORT` | `18081` | Docker sandbox verification. |

If a port conflicts with a different local service, set a different value.
The integration runner stops immediately if its target health endpoint already gives a response.
This check prevents tests and seed data on the wrong Osprey process.

## Local Gate

```bash
VERSION=sandbox-YYYYMMDD \
OSPREY_TEST_PORT=18080 \
DOCKER_PORT=18081 \
./scripts/assure-sandbox.sh
```

This script validates these items:

- JSON examples
- OpenAPI syntax, refs, examples, and route coverage
- Markdown links and PNG assets
- shell script syntax
- regression tests for sandbox-script input and occupied ports
- Go tests, `go vet`, and race tests
- HTTP integration tests
- Docker build metadata, healthcheck, and API smoke behavior

## Public URL Gate

**WARNING:** Run `verify-sandbox.sh` only against a disposable sandbox, or with the approval of the deployment operator. The script changes its target.

The script does these changes and checks:

- It creates or replaces the global rules `sandbox-verification-same-party` and `sandbox-verification-typology`.
- It sends transactions.
- It does checks on a second tenant.

```bash
OSPREY_URL=https://your-osprey-host.example \
TENANT_ID=demo \
OSPREY_ADMIN_TOKEN=replace-with-admin-token \
EXPECTED_STATUS=healthy \
EXPECTED_MODE=detection \
EXPECTED_VERSION=sandbox-YYYYMMDD \
./scripts/verify-sandbox.sh
```

This script verifies these items:

- `/health` and `/ready`
- the expected status, mode, and version, if you supply them
- admin-token protection
- rule creation and immediate activation
- typology creation and immediate activation
- the `NALT` response for a normal transaction
- the `ALRT` response for a same-party transaction
- evaluation and transaction retrieval
- the conflict response for a duplicate transaction
- tenant isolation

You must set `OSPREY_URL` and `OSPREY_ADMIN_TOKEN`.
The script has no default public sandbox target.

**WARNING:** Do not publish a sandbox URL before the two gates pass.
