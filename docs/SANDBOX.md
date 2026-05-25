# Osprey Sandbox Guide

This guide is for customers integrating with the Osprey sandbox API at:

```text
https://sandbox.osprey.opensource.finance
```

The sandbox is a safe environment for validating transaction monitoring flows, rule behavior, typology behavior, tenant isolation, and response handling before using Osprey in a production workflow.

For the shortest customer handoff, start with [docs/CUSTOMER_QUICKSTART.md](CUSTOMER_QUICKSTART.md). Assurance evidence is tracked in [docs/ASSURANCE.md](ASSURANCE.md). The machine-readable API contract is [docs/api/openapi.yaml](api/openapi.yaml). Use it for customer SDK generation, Postman import, contract review, or automated API checks. For authoring guidance, use [docs/RULE_TYPOLOGY_AUTHORING.md](RULE_TYPOLOGY_AUTHORING.md).

## Customer Flow

1. Send a transaction to `POST /evaluate`.
2. Osprey stores the transaction for the provided tenant.
3. Osprey evaluates the transaction against active rules.
4. In compliance mode, Osprey also evaluates typologies.
5. Osprey returns a JSON decision: `ALRT` or `NALT`.
6. Use the returned IDs to fetch the stored transaction or evaluation.

![Osprey sandbox flow](assets/osprey-sandbox-flow.png)

## Required Headers

Every tenant-scoped request must include:

```http
X-Tenant-ID: <tenant-id>
```

JSON requests must include:

```http
Content-Type: application/json
```

Rule and typology mutation endpoints require an admin token when `OSPREY_ADMIN_TOKEN` is configured:

```http
Authorization: Bearer <admin-token>
```

or:

```http
X-Osprey-Admin-Token: <admin-token>
```

## Health Checks

Use these before sending traffic:

```bash
curl -fsS https://sandbox.osprey.opensource.finance/health
curl -fsS https://sandbox.osprey.opensource.finance/ready
```

Expected healthy response:

```json
{
  "mode": "detection",
  "status": "healthy",
  "version": "..."
}
```

In compliance mode, `/ready` returns `503` until typologies are loaded.

## Evaluate a Transaction

The response depends on the rules currently loaded in the sandbox. To reproduce the `ALRT` example below, create the sample same-party rule first or load the starter kit.

```bash
curl -fsS -X POST https://sandbox.osprey.opensource.finance/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo-client" \
  -d @docs/examples/evaluate-alert.json
```

Example response:

```json
{
  "evaluationId": "7f52d4a2-6d8e-4c70-b21f-7f57e3c37f9a",
  "txId": "sandbox-alert-example",
  "status": "ALRT",
  "score": 1,
  "reasons": [
    "Same party transfer detected"
  ],
  "metadata": {
    "traceId": "8dc5b7f1-9bb5-4f94-b8b5-6559e1c6b2e2",
    "ingestMs": 0,
    "totalMs": 4,
    "version": "..."
  }
}
```

Status values:

| Status | Meaning |
|--------|---------|
| `ALRT` | Alert. The transaction matched enough risk signals to require review. |
| `NALT` | No alert. The transaction was evaluated and did not cross the alert threshold. |

## Fetch Evaluation and Transaction Records

Use the IDs returned by `/evaluate`.

```bash
curl -fsS https://sandbox.osprey.opensource.finance/evaluations/7f52d4a2-6d8e-4c70-b21f-7f57e3c37f9a \
  -H "X-Tenant-ID: demo-client"

curl -fsS https://sandbox.osprey.opensource.finance/transactions/sandbox-alert-example \
  -H "X-Tenant-ID: demo-client"
```

The same transaction ID can be reused by different tenants, but it cannot be reused inside the same tenant. A duplicate transaction ID for the same tenant returns `409 Conflict`.

## Create a Rule

Rules use Google CEL expressions. A rule returns a boolean or numeric score. `true` maps to `1.0`; `false` maps to `0.0`.

For rule design workflow and promotion criteria, see [docs/RULE_TYPOLOGY_AUTHORING.md](RULE_TYPOLOGY_AUTHORING.md).

```bash
curl -fsS -X POST https://sandbox.osprey.opensource.finance/rules \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo-client" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json
```

Rules are persisted and loaded into the active engine immediately.

### Rule Fields

| Field | Required | Notes |
|-------|----------|-------|
| `id` | Yes | Stable machine-readable ID. |
| `name` | Yes | Human-readable name. |
| `description` | No | Use this to explain the monitoring intent. |
| `expression` | Yes | CEL expression. |
| `weight` | Yes | Number from `0` to `1`. |
| `enabled` | Yes | Disabled rules are stored but not loaded. |
| `bands` | No | Human-readable reasons for score ranges. |

### CEL Variables

| Variable | Type | Description |
|----------|------|-------------|
| `amount` | double | Transaction amount. |
| `currency` | string | Uppercase currency code. |
| `tx_type` | string | Uppercase transaction type. |
| `debtor_id` | string | Sender/customer ID. |
| `creditor_id` | string | Receiver/merchant ID. |
| `old_balance` | double | Optional value from `metadata.old_balance`. |
| `new_balance` | double | Optional value from `metadata.new_balance`. |
| `velocity_count` | int | Recent transaction count for the entity. |

Common expressions:

```cel
amount > 10000.0
amount >= 9000.0 && amount < 10000.0
debtor_id == creditor_id
old_balance > 0.0 && new_balance == 0.0
velocity_count > 5
tx_type == "CASH_OUT" || tx_type == "TRANSFER"
```

## Create a Typology

Typologies are evaluated only in compliance mode. They combine active rules into a named pattern.

```bash
curl -fsS -X POST https://sandbox.osprey.opensource.finance/typologies \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo-client" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/typology-same-party.json
```

Every `ruleId` must already exist in the active rule engine. Typology weights should normally sum to `1.0`.

## Useful Read Endpoints

```bash
curl -fsS https://sandbox.osprey.opensource.finance/rules \
  -H "X-Tenant-ID: demo-client"

curl -fsS https://sandbox.osprey.opensource.finance/typologies \
  -H "X-Tenant-ID: demo-client"
```

Both endpoints return the active engine state, not merely stored database rows.

## Error Contract

| HTTP Status | Common Cause |
|-------------|--------------|
| `400` | Missing tenant header, invalid JSON, invalid rule/typology shape. |
| `401` | Missing or invalid admin token on mutation endpoint. |
| `404` | Transaction, evaluation, rule, or typology not found. |
| `409` | Duplicate transaction ID for the same tenant. |
| `413` | JSON request body is too large. |
| `500` | Internal persistence or evaluation error. |
| `503` | Repository unavailable, or compliance mode is not ready because typologies are missing. |

## Coolify Deployment

Use the repository Dockerfile and expose port `8080`.

For an operator checklist, use [docs/COOLIFY_SANDBOX_CHECKLIST.md](COOLIFY_SANDBOX_CHECKLIST.md).

Recommended environment variables for a simple sandbox:

```env
OSPREY_MODE=detection
OSPREY_TIER=community
OSPREY_DB_DRIVER=sqlite
OSPREY_SQLITE_PATH=/app/data/osprey.db
OSPREY_ADMIN_TOKEN=<strong-random-token>
OSPREY_DEBUG=false
```

The copy-paste Coolify templates are:

- [docs/coolify-sandbox.env.example](coolify-sandbox.env.example)
- [docs/coolify-sandbox.build-args.example](coolify-sandbox.build-args.example)

Mount a persistent volume at:

```text
/app/data
```

The image creates `/app/data` with ownership for the non-root `osprey` user. Use a persistent Coolify volume for this path so SQLite data, rules, typologies, transactions, and evaluations survive restarts.

Set the public domain to:

```text
sandbox.osprey.opensource.finance
```

Coolify should route HTTPS traffic to container port `8080`.

Recommended Docker build arguments:

```env
VERSION=sandbox-YYYYMMDD
COMMIT=<git-sha>
BUILD_DATE=<utc-build-time>
```

Generate consistent build and verification values with:

```bash
./scripts/print-sandbox-build-args.sh
```

`GET /health` exposes the deployed `version`, so customers and operators can confirm which sandbox image is running.

For compliance-mode sandbox testing, set:

```env
OSPREY_MODE=compliance
```

Then load both rules and typologies:

```bash
OSPREY_URL=https://sandbox.osprey.opensource.finance \
OSPREY_ADMIN_TOKEN=<admin-token> \
./scripts/seed-starter-kit.sh --compliance
```

## Operator Verification

Before deploying a new sandbox image, run the full local assurance gate:

```bash
./scripts/assure-sandbox.sh
```

This requires `curl`, `go`, `jq`, `ruby`, and `docker`. It validates JSON examples, the OpenAPI contract, shell scripts, Go tests, `go vet`, the race detector, HTTP integration tests, Docker build, Docker health, and the Docker-backed sandbox API verifier.

After deployment, run:

```bash
OSPREY_URL=https://sandbox.osprey.opensource.finance \
TENANT_ID=demo-client \
OSPREY_ADMIN_TOKEN=<admin-token> \
EXPECTED_STATUS=healthy \
EXPECTED_MODE=detection \
EXPECTED_VERSION=sandbox-YYYYMMDD \
./scripts/verify-sandbox.sh
```

This verifies:

- `/health`
- `/ready`
- expected health status, mode, and version when provided
- admin-token protection for rule and typology mutation when `OSPREY_ADMIN_TOKEN` is set
- rule creation and immediate activation
- typology creation and immediate activation
- `NALT` response for a normal transaction
- `ALRT` response for a same-party transaction
- evaluation retrieval
- transaction retrieval
- duplicate transaction conflict handling
- tenant isolation for transaction IDs and evaluation reads

Do not hand a sandbox to a customer until this script passes against the public domain.
