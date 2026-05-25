# Coolify Sandbox Checklist

Use this checklist to deploy Osprey as the customer sandbox at:

```text
https://sandbox.osprey.opensource.finance
```

## Pre-Deploy Gate

Run the local assurance gate before creating or updating the Coolify deployment:

```bash
VERSION=sandbox-YYYYMMDD \
./scripts/assure-sandbox.sh
```

This must pass before the sandbox is handed to a customer.

The same gate also runs in GitHub Actions as `Sandbox Assurance` on pull requests, pushes to `main`, and manual dispatch. Treat a failing workflow as a release blocker.

## Coolify Application

| Setting | Value |
|---------|-------|
| Build source | Repository |
| Build type | Dockerfile |
| Dockerfile path | `Dockerfile` |
| Container port | `8080` |
| Public domain | `sandbox.osprey.opensource.finance` |
| HTTPS | Enabled |
| Health path | `/health` |
| Persistent volume | `/app/data` |

## DNS and TLS

Create a DNS record for:

```text
sandbox.osprey.opensource.finance
```

Point it at the Coolify ingress target before customer handoff. Do not share the sandbox until:

```bash
curl -fsS https://sandbox.osprey.opensource.finance/health
```

returns JSON from Osprey over HTTPS. A DNS failure, TLS certificate failure, redirect loop, proxy error, or non-Osprey response is a release blocker.

## Build Arguments

Set these in Coolify build arguments. Use [coolify-sandbox.build-args.example](coolify-sandbox.build-args.example) as the copy source:

```env
VERSION=sandbox-YYYYMMDD
COMMIT=<git-sha>
BUILD_DATE=<utc-build-time>
```

To generate consistent values from the current checkout:

```bash
./scripts/print-sandbox-build-args.sh
```

`GET /health` returns the deployed `version`. Use it to confirm Coolify is serving the expected image.

## Environment Variables

Use these for the default customer sandbox. Use [coolify-sandbox.env.example](coolify-sandbox.env.example) as the copy source:

```env
OSPREY_MODE=detection
OSPREY_TIER=community
OSPREY_DB_DRIVER=sqlite
OSPREY_SQLITE_PATH=/app/data/osprey.db
OSPREY_ADMIN_TOKEN=<strong-random-token>
OSPREY_DEBUG=false
```

Keep `OSPREY_ADMIN_TOKEN` private. Customers need it only if they are allowed to create or update rules and typologies directly.

## Persistent Storage

Mount a persistent volume at:

```text
/app/data
```

This stores the SQLite database, including:

- rules
- typologies
- transactions
- evaluations

Do not deploy a customer sandbox without this volume.

## Optional Compliance Sandbox

For compliance-mode testing:

```env
OSPREY_MODE=compliance
```

After deployment, seed rules and typologies:

```bash
OSPREY_URL=https://sandbox.osprey.opensource.finance \
OSPREY_ADMIN_TOKEN=<admin-token> \
./scripts/seed-starter-kit.sh --compliance
```

Compliance mode returns `503` from `/ready` and `/evaluate` until typologies are loaded.

## Post-Deploy Verification

After Coolify reports the app is running, verify the public domain:

```bash
OSPREY_URL=https://sandbox.osprey.opensource.finance \
TENANT_ID=demo-client \
OSPREY_ADMIN_TOKEN=<admin-token> \
EXPECTED_STATUS=healthy \
EXPECTED_MODE=detection \
EXPECTED_VERSION=sandbox-YYYYMMDD \
./scripts/verify-sandbox.sh
```

Then confirm the deployed version:

```bash
curl -fsS https://sandbox.osprey.opensource.finance/health | jq '{status, mode, version}'
```

Expected:

```json
{
  "status": "healthy",
  "mode": "detection",
  "version": "sandbox-YYYYMMDD"
}
```

## Customer Handoff

Share these with customers:

- [Customer quickstart](CUSTOMER_QUICKSTART.md)
- [Sandbox guide](SANDBOX.md)
- [Assurance evidence](ASSURANCE.md)
- [OpenAPI contract](api/openapi.yaml)
- [Example payloads](examples/)

Do not share the admin token unless the customer is expected to author rules or typologies.

## Rollback Signals

Rollback or pause handoff if any of these fail:

- `sandbox.osprey.opensource.finance` does not resolve publicly.
- HTTPS is not valid for `sandbox.osprey.opensource.finance`.
- Coolify container health is unhealthy.
- `GET /health` does not return `status: "healthy"`.
- `GET /health` returns an unexpected `version`.
- `./scripts/verify-sandbox.sh` fails against the public domain.
- `/ready` returns `503` in detection mode.
- Rule or typology mutation returns `401` with the expected admin token.
- Rule or typology mutation succeeds without an admin token when `OSPREY_ADMIN_TOKEN` is configured.
