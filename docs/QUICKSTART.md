# Quickstart

This path proves the core Osprey loop:

1. Start Osprey.
2. Evaluate a normal transaction.
3. Add a rule.
4. Evaluate a transaction that triggers the rule.

## 1. Start Osprey

```bash
export OSPREY_ADMIN_TOKEN=local-admin-token
go run ./cmd/osprey
```

The API listens on:

```text
http://localhost:8080
```

## 2. Check Health

```bash
curl -fsS http://localhost:8080/health
curl -fsS http://localhost:8080/ready
```

Healthy detection-mode response:

```json
{
  "status": "healthy",
  "mode": "detection"
}
```

## 3. Evaluate a Normal Transaction

```bash
curl -fsS -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -d @docs/examples/evaluate-normal.json
```

Important response fields:

| Field | Meaning |
|-------|---------|
| `evaluationId` | Stored decision ID. |
| `txId` | Transaction ID. |
| `status` | `NALT` or `ALRT`. |
| `score` | Risk score from `0` to `1`. |
| `reasons` | Rule reasons shown to reviewers. |
| `metadata.traceId` | Request trace ID. |

## 4. Add a Rule

```bash
curl -fsS -X POST http://localhost:8080/rules \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json
```

The sample rule alerts when sender and receiver are the same entity:

```cel
debtor_id == creditor_id
```

Rules are active immediately after a successful write.

## 5. Trigger the Rule

```bash
curl -fsS -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -d @docs/examples/evaluate-alert.json
```

Expected decision:

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
