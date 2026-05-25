# Customer Quickstart

Use this page to test the Osprey sandbox in a few minutes.

Base URL:

```text
https://sandbox.osprey.opensource.finance
```

Required header for every tenant-scoped request:

```http
X-Tenant-ID: demo-client
```

## 1. Check the Sandbox

```bash
curl -fsS https://sandbox.osprey.opensource.finance/health
curl -fsS https://sandbox.osprey.opensource.finance/ready
```

Expected:

```json
{
  "status": "healthy",
  "mode": "detection",
  "version": "sandbox-YYYYMMDD"
}
```

## 2. Evaluate a Transaction

```bash
curl -fsS -X POST https://sandbox.osprey.opensource.finance/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo-client" \
  -d @docs/examples/evaluate-normal.json
```

The response includes:

| Field | Meaning |
|-------|---------|
| `evaluationId` | ID for the stored decision. |
| `txId` | Transaction ID. |
| `status` | `NALT` for no alert, `ALRT` for alert. |
| `score` | Risk score from `0` to `1`. |
| `reasons` | Human-readable rule reasons. |
| `metadata.traceId` | Request trace ID for support. |

## 3. Create a Rule

Rule and typology writes require an admin token when the sandbox is protected.

```bash
curl -fsS -X POST https://sandbox.osprey.opensource.finance/rules \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo-client" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json
```

This sample rule alerts when:

```cel
debtor_id == creditor_id
```

Rules are active immediately after a successful response.

## 4. Test an Alert

```bash
curl -fsS -X POST https://sandbox.osprey.opensource.finance/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo-client" \
  -d @docs/examples/evaluate-alert.json
```

Expected:

```json
{
  "status": "ALRT",
  "reasons": ["Same party transfer detected"]
}
```

## 5. Create a Typology

Typologies combine active rules into a named pattern. They affect decisions only when Osprey runs in compliance mode.

```bash
curl -fsS -X POST https://sandbox.osprey.opensource.finance/typologies \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo-client" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/typology-same-party.json
```

Every typology `ruleId` must already exist in `GET /rules`.

## Keep Going

- Full sandbox guide: [SANDBOX.md](SANDBOX.md)
- Rule and typology authoring: [RULE_TYPOLOGY_AUTHORING.md](RULE_TYPOLOGY_AUTHORING.md)
- OpenAPI contract: [api/openapi.yaml](api/openapi.yaml)
- Example payloads: [examples/](examples/)
