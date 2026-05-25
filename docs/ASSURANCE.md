# Sandbox Assurance

Use this page as the evidence map for Osprey sandbox handoff.

## Required Gates

Local requirements:

```text
curl, go, jq, ruby, docker
```

Run before deploying or updating the customer sandbox:

```bash
VERSION=sandbox-YYYYMMDD ./scripts/assure-sandbox.sh
```

Run after the public sandbox is deployed:

```bash
OSPREY_URL=https://sandbox.osprey.opensource.finance \
TENANT_ID=demo-client \
OSPREY_ADMIN_TOKEN=replace-with-admin-token \
EXPECTED_STATUS=healthy \
EXPECTED_MODE=detection \
EXPECTED_VERSION=sandbox-YYYYMMDD \
./scripts/verify-sandbox.sh
```

## Evidence Matrix

| Requirement | Evidence |
|-------------|----------|
| Customer examples are valid JSON | `assure-sandbox.sh` runs `jq empty docs/examples/*.json`. |
| API contract is internally consistent | `assure-sandbox.sh` parses `docs/api/openapi.yaml`, resolves local refs/examples, validates external request examples against their declared schemas, checks OpenAPI routes match registered server routes, and verifies admin-protected routes document both admin token mechanisms. |
| Readiness responses are modeled precisely | `docs/api/openapi.yaml` defines separate `ReadyResponse` and `NotReadyResponse` schemas for `/ready` `200` and `503`. |
| CI gate is wired correctly | `assure-sandbox.sh` validates `.github/workflows/sandbox-assurance.yml` runs the assurance gate and installs required tools. |
| Customer docs do not have broken local links | `assure-sandbox.sh` validates `README.md` and `docs/**/*.md` links and PNG assets. |
| Code compiles and core tests pass | `assure-sandbox.sh` runs `go test ./...`, `go vet ./...`, and `go test -race ./...`. |
| HTTP behavior works end-to-end | `assure-sandbox.sh` runs `scripts/test-integration.sh`. |
| Docker image is deployable | `assure-sandbox.sh` builds the Dockerfile, verifies image metadata for non-root user, exposed port, volume, and healthcheck, then runs the image with a named `/app/data` volume. |
| Deployed build identity is correct | `verify-sandbox.sh` checks expected status, mode, and version from `/health`. |
| Runtime readiness contract is correct | `verify-sandbox.sh` requires `/ready` to return HTTP `200` with `ready=true`. |
| Admin writes are protected | `verify-sandbox.sh` confirms unauthenticated rule and typology writes return `401`, and that the documented `X-Osprey-Admin-Token` header is accepted. |
| Authorized rule and typology writes work | `verify-sandbox.sh` creates a rule and typology and verifies both are active. |
| Decisions are correct for baseline payloads | `verify-sandbox.sh` expects `NALT` for normal and `ALRT` for same-party examples. |
| Persistence works | `verify-sandbox.sh` fetches the stored evaluation and transaction. |
| Idempotency guard works | `verify-sandbox.sh` expects duplicate transaction ID to return `409`. |
| Tenant isolation works | `verify-sandbox.sh` reuses a transaction ID in another tenant and confirms the original evaluation is not readable there. |

Do not hand the sandbox to a customer until both required gates pass.
