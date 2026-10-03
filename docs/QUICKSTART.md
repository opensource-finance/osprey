# Quickstart

This procedure shows the core Osprey loop:

1. Start Osprey.
2. Evaluate a normal transaction.
3. Add a rule.
4. Evaluate a transaction that triggers the rule.

## 1. Start Osprey

Set the admin token and start the server:

```bash
export OSPREY_ADMIN_TOKEN=local-admin-token
go run ./cmd/osprey
```

The API listens on this address:

```text
http://localhost:8080
```

## 2. Check Health

Send a request to the health and the ready endpoints:

```bash
curl -fsS http://localhost:8080/health
curl -fsS http://localhost:8080/ready
```

In detection mode, a healthy server gives this response:

```json
{
  "status": "healthy",
  "mode": "detection"
}
```

## 3. Evaluate a Normal Transaction

Send the normal sample transaction:

```bash
curl -fsS -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -d @docs/examples/evaluate-normal.json
```

Important response fields:

| Field | Meaning |
|-------|---------|
| `evaluationId` | The ID of the stored decision. |
| `txId` | Transaction ID. |
| `status` | `NALT` or `ALRT`. |
| `score` | Risk score from `0` to `1`. |
| `reasons` | Rule reasons that reviewers see. |
| `metadata.traceId` | Request trace ID. |

## 4. Add a Rule

**WARNING:** Do not share `OSPREY_ADMIN_TOKEN`. The token gives write access to all rules.

Send the sample rule with the admin token:

```bash
curl -fsS -X POST http://localhost:8080/rules \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json
```

The sample rule gives an alert if the sender and the receiver are the same entity:

```cel
debtor_id == creditor_id
```

A rule becomes active immediately after a successful write.

## 5. Trigger the Rule

Send the sample transaction that triggers the rule:

```bash
curl -fsS -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -d @docs/examples/evaluate-alert.json
```

Osprey returns this decision:

```json
{
  "status": "ALRT",
  "reasons": ["Same party transfer detected"]
}
```

## Next

- [Sandbox and API guide](SANDBOX.md)
- [Rule and typology authoring](RULE_TYPOLOGY_AUTHORING.md)
- [Starter kit rules](STARTER_KIT.md)
- [OpenAPI contract](api/openapi.yaml)
