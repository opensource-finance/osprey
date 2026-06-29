# Sandbox and API Guide

Use this guide when Osprey is running behind a shared or remote URL.

Set your target once:

```bash
export OSPREY_URL=http://localhost:8080
export TENANT_ID=demo
```

For a hosted sandbox, set `OSPREY_URL` to that public URL.

## Request Rules

Every tenant-scoped request needs:

```http
X-Tenant-ID: <tenant-id>
```

JSON requests need:

```http
Content-Type: application/json
```

Rule and typology writes need an admin token:

```http
Authorization: Bearer <admin-token>
```

or:

```http
X-Osprey-Admin-Token: <admin-token>
```

## Flow

1. Check `/health` and `/ready`.
2. Send a transaction to `POST /evaluate`.
3. Add rules with `POST /rules`.
4. In compliance mode, add typologies with `POST /typologies`.
5. Use `evaluationId` and `txId` to fetch stored records.

![Osprey sandbox flow](assets/osprey-sandbox-flow.png)

## Health

```bash
curl -fsS "$OSPREY_URL/health"
curl -fsS "$OSPREY_URL/ready"
```

In detection mode, both should be healthy before sending traffic. In compliance mode, `/ready` returns `503` until typologies are loaded.

## Evaluate

```bash
curl -fsS -X POST "$OSPREY_URL/evaluate" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d @docs/examples/evaluate-normal.json
```

Decision values:

| Status | Meaning |
|--------|---------|
| `NALT` | No alert. |
| `ALRT` | Alert. |

Example alert response:

```json
{
  "evaluationId": "7f52d4a2-6d8e-4c70-b21f-7f57e3c37f9a",
  "txId": "sandbox-alert-example",
  "status": "ALRT",
  "score": 1,
  "reasons": ["Same party transfer detected"],
  "metadata": {
    "traceId": "8dc5b7f1-9bb5-4f94-b8b5-6559e1c6b2e2",
    "ingestMs": 1,
    "totalMs": 4,
    "version": "..."
  }
}
```

## Fetch Stored Records

```bash
curl -fsS "$OSPREY_URL/evaluations/<evaluation-id>" \
  -H "X-Tenant-ID: $TENANT_ID"

curl -fsS "$OSPREY_URL/transactions/<transaction-id>" \
  -H "X-Tenant-ID: $TENANT_ID"
```

Transaction IDs are unique per tenant. Reusing a transaction ID in the same tenant returns `409 Conflict`.

## Create a Rule

Rules use CEL expressions. A rule returns a boolean or numeric score. `true` maps to `1.0`; `false` maps to `0.0`.

```bash
curl -fsS -X POST "$OSPREY_URL/rules" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json
```

Rule fields:

| Field | Required | Notes |
|-------|----------|-------|
| `id` | Yes | Stable machine-readable ID. |
| `name` | Yes | Human-readable name. |
| `expression` | Yes | CEL expression. |
| `weight` | Yes | Number from `0` to `1`. |
| `enabled` | Yes | Disabled rules are stored but not active. |
| `bands` | No | Reasons for score ranges. |

Common CEL variables:

| Variable | Source |
|----------|--------|
| `amount` | `amount.value` |
| `currency` | `amount.currency`, uppercase |
| `tx_type` | `type`, uppercase |
| `debtor_id` | `debtor.id` |
| `creditor_id` | `creditor.id` |
| `old_balance` | `metadata.old_balance` |
| `new_balance` | `metadata.new_balance` |
| `velocity_count` | Recent transaction count for the entity |

Common expressions:

```cel
amount > 10000.0
amount >= 9000.0 && amount < 10000.0
debtor_id == creditor_id
old_balance > 0.0 && new_balance == 0.0
velocity_count > 5
tx_type == "CASH_OUT" || tx_type == "TRANSFER"
```

## Update or Delete a Rule

```bash
# Update an existing rule (URL id is authoritative)
curl -fsS -X PUT "$OSPREY_URL/rules/my-rule" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json

# Delete a rule
curl -fsS -X DELETE "$OSPREY_URL/rules/my-rule" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN"
```

Both apply to the active engine immediately. Deleting a rule that a loaded typology still references returns `409`; remove it from the typology first.

## Create a Typology

Typologies are evaluated only in compliance mode. They combine active rules into a named pattern.

```bash
curl -fsS -X POST "$OSPREY_URL/typologies" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/typology-same-party.json
```

Every `ruleId` must already exist in `GET /rules`.

## Read Active Config

```bash
curl -fsS "$OSPREY_URL/rules" \
  -H "X-Tenant-ID: $TENANT_ID"

curl -fsS "$OSPREY_URL/typologies" \
  -H "X-Tenant-ID: $TENANT_ID"
```

These endpoints return active engine state.

## Errors

| Status | Common Cause |
|--------|--------------|
| `400` | Missing tenant header, invalid JSON, invalid rule or typology. |
| `401` | Missing or invalid admin token on a write endpoint. |
| `404` | Record not found. |
| `409` | Duplicate transaction ID for the same tenant, or deleting a rule still referenced by a typology. |
| `413` | JSON body is too large. |
| `429` | Per-tenant rate limit exceeded (only when `OSPREY_RATE_LIMIT_RPS` is set). |
| `500` | Persistence or evaluation error. |
| `503` | Repository unavailable, or compliance mode is missing typologies. |

## Deploy a Simple Sandbox

Use the repository Dockerfile and expose port `8080`.

```env
OSPREY_MODE=detection
OSPREY_TIER=community
OSPREY_DB_DRIVER=sqlite
OSPREY_SQLITE_PATH=/app/data/osprey.db
OSPREY_ADMIN_TOKEN=replace-with-strong-random-token
```

Mount persistent storage at:

```text
/app/data
```

The admin token is trusted: anyone who has it can author rules, so share it only
with sandbox users you trust. For a public sandbox, optionally cap per-tenant
request rate (leave unset for load testing):

```env
OSPREY_RATE_LIMIT_RPS=50
```

Verify before sharing a URL:

```bash
./scripts/assure-sandbox.sh

OSPREY_URL=https://your-osprey-host.example \
TENANT_ID=demo \
OSPREY_ADMIN_TOKEN=replace-with-admin-token \
EXPECTED_STATUS=healthy \
EXPECTED_MODE=detection \
./scripts/verify-sandbox.sh
```
