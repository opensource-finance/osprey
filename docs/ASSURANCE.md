# Sandbox Assurance

Use these checks before publishing or updating a shared Osprey sandbox.

## Required Tools

```text
curl, go, jq, ruby, docker
```

## Local Gate

```bash
VERSION=sandbox-YYYYMMDD ./scripts/assure-sandbox.sh
```

This validates:

- JSON examples
- OpenAPI syntax, refs, examples, and route coverage
- Markdown links and PNG assets
- shell script syntax
- Go tests, `go vet`, and race tests
- HTTP integration tests
- Docker build metadata, healthcheck, and API smoke behavior

## Public URL Gate

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

Do not publish a sandbox URL until both gates pass.
