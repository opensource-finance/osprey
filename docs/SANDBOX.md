# Sandbox and API Guide

Use this guide to:

1. deploy a local Osprey sandbox with Docker, or
2. call an existing sandbox URL supplied by its owner.

Osprey does not currently provide a maintained public sandbox URL. To use a
remote sandbox, ask its owner for the base URL. You also need the admin token to
change rules or typologies.

## Set Up with an AI Agent

Copy this prompt into an AI coding agent that can run shell commands:

```text
Set up an Osprey sandbox on my computer.

Use this guide as the source of truth:
https://github.com/opensource-finance/osprey/blob/main/docs/SANDBOX.md

Rules:
1. Read the current guide before running commands.
2. Create a local Docker sandbox only. Do not expose it to the public internet.
3. Check for Git, Docker, curl, and OpenSSL first. Check that Docker is running.
   If something is missing, stop and tell me what I need to install or start.
4. Use a new clone of https://github.com/opensource-finance/osprey.git. If the
   target folder already exists, ask before using or changing it.
5. Before creating Docker resources, check whether the container name, volume
   name, image name, or host port is already in use. Do not delete or replace
   existing resources. Ask me what to do if there is a name conflict.
6. Follow the "Deploy a Local Docker Sandbox" section. If port 8080 is busy,
   choose an unused local port. Bind the service to 127.0.0.1 only.
7. Generate a new admin token. Save it in .env.sandbox.local as the guide says.
   Never print the token in chat, logs, command output, or the final report.
8. Do not edit Osprey source files, delete data, or push anything to Git.
9. Verify /health and /ready. Then run the guide's normal transaction, create
   the sample rule, and run the alert transaction. Do not claim success unless
   all checks pass.
10. If a command fails, stop, keep useful logs, and explain the exact failure in
    plain English. Do not hide the error or use an unsafe workaround.
11. At the end, report the local URL, container name, volume name, settings file
    path, and test results. Include the commands to stop and start the sandbox.
    Do not include the admin token.
```

The prompt points to this guide instead of copying its commands. This keeps one
setup path and prevents the prompt from becoming outdated.

## Prerequisites

For a local Docker sandbox, install:

- Git
- Docker with a running Docker daemon
- `curl`
- OpenSSL, used below to generate an admin token

The API examples are self-contained and work from any directory. The automated
verification scripts require a repository clone and additional tools described
in [Sandbox Assurance](ASSURANCE.md).

## Deploy a Local Docker Sandbox

### 1. Clone the repository

```bash
git clone https://github.com/opensource-finance/osprey.git
cd osprey
```

Run the remaining deployment and verification commands from the repository
root.

### 2. Configure the sandbox

```bash
export OSPREY_HOST_PORT=8080
export OSPREY_ADMIN_TOKEN="$(openssl rand -hex 32)"

umask 077
cat > .env.sandbox.local <<EOF
OSPREY_HOST_PORT=$OSPREY_HOST_PORT
OSPREY_URL=http://127.0.0.1:$OSPREY_HOST_PORT
TENANT_ID=demo
OSPREY_MODE=detection
OSPREY_TIER=community
OSPREY_DB_DRIVER=sqlite
OSPREY_SQLITE_PATH=/app/data/osprey.db
OSPREY_ADMIN_TOKEN=$OSPREY_ADMIN_TOKEN
OSPREY_RATE_LIMIT_RPS=50
EOF
chmod 600 .env.sandbox.local

set -a
. ./.env.sandbox.local
set +a
```

`OSPREY_HOST_PORT` must be unused. If port `8080` is already occupied, choose
another host port, such as `18080`. Osprey still listens on port `8080` inside
the container.

Keep `OSPREY_ADMIN_TOKEN` private. Anyone who has it can replace the active rule
and typology configuration for every tenant.

`.env.sandbox.local` is the source of truth for this local sandbox. Git ignores
the file. Do not commit or share it. In a new terminal, load it again with:

```bash
set -a
. ./.env.sandbox.local
set +a
```

### 3. Build and run Osprey

```bash
docker build -t osprey-sandbox:local .
docker volume create osprey-sandbox-data

docker run -d \
  --name osprey-sandbox \
  -p "127.0.0.1:${OSPREY_HOST_PORT}:8080" \
  --env-file .env.sandbox.local \
  -v osprey-sandbox-data:/app/data \
  osprey-sandbox:local
```

The named Docker volume persists rules, typologies, transactions, and
evaluations across container restarts.

Remove `OSPREY_RATE_LIMIT_RPS` from the settings file for load testing. Before
sharing a sandbox, choose a request limit that fits your use case.

### 4. Check the deployment

```bash
curl --fail-with-body --silent --show-error "$OSPREY_URL/health"
curl --fail-with-body --silent --show-error "$OSPREY_URL/ready"
```

In detection mode, both endpoints should return HTTP `200`. In compliance mode,
`/ready` returns `503` until typologies are loaded.

Useful lifecycle commands:

```bash
docker logs osprey-sandbox
docker stop osprey-sandbox
docker start osprey-sandbox
```

To remove the container while preserving its data:

```bash
docker rm -f osprey-sandbox
```

`docker volume rm osprey-sandbox-data` permanently deletes the sandbox data.

## Connect to an Existing Sandbox

Set the URL and a tenant identifier supplied or approved by the operator:

```bash
export OSPREY_URL=https://your-osprey-host.example
export TENANT_ID=demo
```

To create, update, delete, or reload rules and typologies, also set the admin
token supplied by the sandbox owner:

```bash
export OSPREY_ADMIN_TOKEN=replace-with-operator-provided-token
```

Evaluation and read requests do not require the admin token.

## Request Rules

Every request for one tenant needs:

```http
X-Tenant-ID: <tenant-id>
```

JSON requests also need:

```http
Content-Type: application/json
```

Changes to rules and typologies accept either admin-token header:

```http
Authorization: Bearer <admin-token>
```

```http
X-Osprey-Admin-Token: <admin-token>
```

Transactions and evaluations are separate for each tenant. Rules and typologies
apply to every tenant. The server returns `tenantId: "*"` for them. Changing a
rule or typology changes results for all tenants. These requests still need the
tenant header so Osprey can identify, limit, and log the request.

## API Flow

1. Check `/health` and `/ready`.
2. Send a transaction to `POST /evaluate`.
3. Add rules with `POST /rules` if you have the admin token.
4. In compliance mode, add typologies with `POST /typologies`.
5. Use `evaluationId` and `txId` to fetch stored records.

![Osprey sandbox flow](assets/osprey-sandbox-flow.png)

## Health

```bash
curl --fail-with-body --silent --show-error "$OSPREY_URL/health"
curl --fail-with-body --silent --show-error "$OSPREY_URL/ready"
```

## Evaluate a Transaction

The generated transaction ID makes this example safe to repeat:

```bash
export NORMAL_TX_ID="sandbox-normal-$(date +%s)"

curl --fail-with-body --silent --show-error \
  -X POST "$OSPREY_URL/evaluate" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  --data-binary @- <<JSON
{
  "id": "$NORMAL_TX_ID",
  "type": "TRANSFER",
  "debtor": {"id": "user-a", "accountId": "acct-a"},
  "creditor": {"id": "merchant-b", "accountId": "acct-b"},
  "amount": {"value": 125, "currency": "USD"},
  "timestamp": "2026-05-25T09:15:30Z"
}
JSON
```

Decision values:

| Status | Meaning |
|--------|---------|
| `NALT` | No alert. |
| `ALRT` | Alert. |

Important response fields include `evaluationId`, `txId`, `status`, `score`,
`reasons`, and `metadata.traceId`.

## Fetch Stored Records

Copy the IDs from the evaluation response:

```bash
curl --fail-with-body --silent --show-error \
  "$OSPREY_URL/evaluations/<evaluation-id>" \
  -H "X-Tenant-ID: $TENANT_ID"

curl --fail-with-body --silent --show-error \
  "$OSPREY_URL/transactions/<transaction-id>" \
  -H "X-Tenant-ID: $TENANT_ID"
```

Transaction IDs are unique per tenant. Reusing an ID in the same tenant returns
`409 Conflict`; the same ID may be used by a different tenant.

## Create a Rule

This operation requires `OSPREY_ADMIN_TOKEN` and changes the global active
configuration.

```bash
curl --fail-with-body --silent --show-error \
  -X POST "$OSPREY_URL/rules" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  --data-binary @- <<'JSON'
{
  "id": "sandbox-verification-same-party",
  "name": "Sandbox Verification Same Party",
  "description": "Detects same-entity transfers.",
  "expression": "debtor_id == creditor_id",
  "weight": 1,
  "enabled": true,
  "bands": [
    {
      "lowerLimit": 1,
      "subRuleRef": ".fail",
      "reason": "Same party transfer detected"
    },
    {
      "lowerLimit": 0,
      "upperLimit": 1,
      "subRuleRef": ".pass",
      "reason": "Different parties"
    }
  ]
}
JSON
```

Rule request fields:

| Field | Required | Notes |
|-------|----------|-------|
| `id` | Yes | Stable machine-readable ID. |
| `name` | Yes | Human-readable name. |
| `description` | No | Purpose of the rule. |
| `expression` | Yes | CEL expression. |
| `weight` | Yes | Number from `0` to `1`. |
| `enabled` | Yes | Disabled rules are stored but not active. |
| `bands` | No | Reasons for score ranges. |

The response also contains server-assigned `tenantId: "*"` and `version`
fields. Do not send either field in the request.

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

See [Rule and Typology Authoring](RULE_TYPOLOGY_AUTHORING.md) for the complete
variable list and authoring guidance.

## Trigger the Rule

```bash
export ALERT_TX_ID="sandbox-alert-$(date +%s)"

curl --fail-with-body --silent --show-error \
  -X POST "$OSPREY_URL/evaluate" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  --data-binary @- <<JSON
{
  "id": "$ALERT_TX_ID",
  "type": "TRANSFER",
  "debtor": {"id": "user-alert", "accountId": "acct-alert-a"},
  "creditor": {"id": "user-alert", "accountId": "acct-alert-b"},
  "amount": {"value": 125, "currency": "USD"},
  "timestamp": "2026-05-25T09:16:30Z"
}
JSON
```

The expected decision is `ALRT` with reason `Same party transfer detected`.

## Update a Rule

For an update, the ID in the URL wins. The request body does not need an `id`
field:

```bash
curl --fail-with-body --silent --show-error \
  -X PUT "$OSPREY_URL/rules/sandbox-verification-same-party" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  --data-binary @- <<'JSON'
{
  "name": "Sandbox Verification Same Party",
  "description": "Detects same-entity transfers.",
  "expression": "debtor_id == creditor_id",
  "weight": 1,
  "enabled": true,
  "bands": [
    {
      "lowerLimit": 1,
      "subRuleRef": ".fail",
      "reason": "Same party transfer detected"
    },
    {
      "lowerLimit": 0,
      "upperLimit": 1,
      "subRuleRef": ".pass",
      "reason": "Different parties"
    }
  ]
}
JSON
```

Writes apply to the active engine immediately.

## Create a Typology

Typologies affect decisions only in compliance mode. Creating one changes every
tenant, so it requires the admin token. Every `ruleId` must already appear in
`GET /rules`.

```bash
curl --fail-with-body --silent --show-error \
  -X POST "$OSPREY_URL/typologies" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  --data-binary @- <<'JSON'
{
  "id": "sandbox-verification-typology",
  "name": "Sandbox Verification Typology",
  "description": "Groups the sample same-party rule.",
  "alertThreshold": 0.5,
  "enabled": true,
  "rules": [
    {"ruleId": "sandbox-verification-same-party", "weight": 1}
  ]
}
JSON
```

## Read Active Configuration

```bash
curl --fail-with-body --silent --show-error \
  "$OSPREY_URL/rules" \
  -H "X-Tenant-ID: $TENANT_ID"

curl --fail-with-body --silent --show-error \
  "$OSPREY_URL/typologies" \
  -H "X-Tenant-ID: $TENANT_ID"
```

These endpoints return the current rules and typologies for all tenants.

## Remove the Sample Configuration

Delete the typology before its referenced rule:

```bash
curl --fail-with-body --silent --show-error \
  -X DELETE "$OSPREY_URL/typologies/sandbox-verification-typology" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN"

curl --fail-with-body --silent --show-error \
  -X DELETE "$OSPREY_URL/rules/sandbox-verification-same-party" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN"
```

Deleting a rule still referenced by a loaded typology returns `409 Conflict`.

## Verify Before Sharing a URL

Run the automated commands from a repository clone. The full assurance gate
requires `curl`, Go 1.26 or later, `jq`, Ruby, and Docker:

```bash
OSPREY_TEST_PORT=18080 \
DOCKER_PORT=18081 \
./scripts/assure-sandbox.sh
```

Both ports must be unused. The integration runner now refuses an occupied health
port instead of testing whichever service already owns it.

The public URL verifier changes the sandbox. It creates or replaces the global
rules `sandbox-verification-same-party` and
`sandbox-verification-typology`, submits transactions, and checks a second
tenant. Run it only against a temporary sandbox or one whose owner has approved
those changes.

```bash
OSPREY_URL=https://your-osprey-host.example \
TENANT_ID=demo \
OSPREY_ADMIN_TOKEN=replace-with-admin-token \
EXPECTED_STATUS=healthy \
EXPECTED_MODE=detection \
./scripts/verify-sandbox.sh
```

`OSPREY_URL` is required. The verifier has no default URL.

## Errors

| Status | Common Cause |
|--------|--------------|
| `400` | Missing tenant header, invalid JSON, invalid rule or typology. |
| `401` | Missing or invalid admin token on a write endpoint. |
| `404` | Record not found. |
| `409` | Duplicate transaction ID, or deleting a referenced rule. |
| `413` | JSON body is too large. |
| `429` | Per-tenant rate limit exceeded when rate limiting is enabled. |
| `500` | Persistence or evaluation error. |
| `503` | Repository unavailable, or compliance mode is missing typologies. |
