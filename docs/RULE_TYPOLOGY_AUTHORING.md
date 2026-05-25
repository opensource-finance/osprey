# Rule and Typology Authoring

Use this guide to design, test, and promote Osprey sandbox rules and typologies.

## Authoring Flow

1. Define the risk signal in plain language.
2. Map the signal to available transaction fields.
3. Write one CEL rule for one detection idea.
4. Add bands with review reasons.
5. Test against one expected `NALT` transaction and one expected `ALRT` transaction.
6. Combine related rules into a typology only when the pattern needs multiple signals.
7. Promote only after the sandbox verifier and customer-specific test cases pass.

## Rule Design Checklist

Every rule should answer:

| Question | Good Answer |
|----------|-------------|
| What risk does it detect? | One specific signal, not a broad category. |
| Which fields does it use? | Fields available in `/evaluate` or `metadata`. |
| What should trigger review? | A precise CEL expression. |
| How strong is the signal? | `weight` from `0` to `1`. |
| What should the reviewer see? | A concise `reason` in the matching band. |

Avoid rules that mix unrelated ideas. Prefer three simple rules over one opaque expression.

## Available CEL Variables

| Variable | Type | Source |
|----------|------|--------|
| `amount` | double | `amount.value` |
| `currency` | string | `amount.currency`, normalized uppercase |
| `tx_type` | string | `type`, normalized uppercase |
| `debtor_id` | string | `debtor.id` |
| `creditor_id` | string | `creditor.id` |
| `old_balance` | double | `metadata.old_balance` |
| `new_balance` | double | `metadata.new_balance` |
| `velocity_count` | int | Recent transaction count for the entity |

## Rule Example

```json
{
  "id": "sandbox-verification-same-party",
  "name": "Sandbox Verification Same Party",
  "description": "Deterministic verification rule for sandbox readiness checks.",
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
```

Create it:

```bash
curl -fsS -X POST https://sandbox.osprey.opensource.finance/rules \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo-client" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json
```

Rules are active immediately after a successful response.

## Common Rule Expressions

```cel
amount > 10000.0
amount >= 9000.0 && amount < 10000.0
debtor_id == creditor_id
old_balance > 0.0 && new_balance == 0.0
velocity_count > 5
tx_type == "CASH_OUT" || tx_type == "TRANSFER"
```

## Typology Design Checklist

Create a typology when several rules together describe a higher-level pattern.

| Question | Good Answer |
|----------|-------------|
| What pattern does it represent? | Structuring, mule activity, account takeover, rapid movement. |
| Which rules contribute? | Existing active rule IDs. |
| Are weights explainable? | Weights reflect relative signal strength and usually sum to `1.0`. |
| What threshold triggers? | `alertThreshold` from `0` exclusive to `1` inclusive. |
| Is compliance mode enabled? | Typologies affect decisions only in compliance mode. |

## Typology Example

```json
{
  "id": "sandbox-verification-typology",
  "name": "Sandbox Verification Typology",
  "description": "Simple typology used to verify sandbox typology authoring.",
  "alertThreshold": 0.5,
  "enabled": true,
  "rules": [
    {
      "ruleId": "sandbox-verification-same-party",
      "weight": 1
    }
  ]
}
```

Create it:

```bash
curl -fsS -X POST https://sandbox.osprey.opensource.finance/typologies \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo-client" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/typology-same-party.json
```

The referenced `ruleId` must already be active.

## Testing Expectations

At minimum, test every new rule with:

| Test | Expected |
|------|----------|
| Normal transaction | `NALT` |
| Trigger transaction | `ALRT` or expected score/reason change |
| Missing required field | `400` |
| Duplicate transaction ID | `409` |

Use the sandbox verifier as a baseline:

```bash
OSPREY_URL=https://sandbox.osprey.opensource.finance \
TENANT_ID=demo-client \
OSPREY_ADMIN_TOKEN=replace-with-admin-token \
./scripts/verify-sandbox.sh
```

## Promotion Criteria

A rule or typology is ready for customer sandbox use when:

- Its expression is small enough to explain in one sentence.
- It has at least one passing and one triggering test transaction.
- The response `reasons` are understandable to an operator.
- It does not depend on fields the customer cannot send.
- It appears in `GET /rules` or `GET /typologies` after creation.
- `./scripts/verify-sandbox.sh` passes against the target sandbox.
