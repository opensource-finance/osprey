# Rule and Typology Authoring

Osprey rules should be small, explainable, and testable. A good rule detects one risk signal.

## Authoring Flow

1. Write the risk signal in plain language.
2. Map it to fields Osprey receives in `/evaluate`.
3. Write one CEL expression.
4. Add clear review reasons in `bands`.
5. Test one expected `NALT` transaction and one expected `ALRT` transaction.
6. Group rules into a typology only when a pattern needs multiple signals.

## Rule Checklist

| Question | Good Answer |
|----------|-------------|
| What risk does this detect? | One specific signal. |
| Which fields does it use? | Fields from the transaction or `metadata`. |
| What triggers review? | A precise CEL expression. |
| How strong is it? | A `weight` from `0` to `1`. |
| What should an operator see? | A concise reason. |

Prefer several simple rules over one opaque expression.

## CEL Variables

| Variable | Type | Source |
|----------|------|--------|
| `amount` | double | `amount.value` |
| `currency` | string | `amount.currency`, uppercase |
| `tx_type` | string | `type`, uppercase |
| `debtor_id` | string | `debtor.id` |
| `creditor_id` | string | `creditor.id` |
| `old_balance` | double | `metadata.old_balance` |
| `new_balance` | double | `metadata.new_balance` |
| `velocity_count` | int | Recent transaction count for the debtor in the window |
| `velocity_amount_sum` | double | Sum of recent transaction amounts for the debtor in the window |
| `velocity_distinct_creditors` | int | Distinct counterparties the debtor transacted with in the window |

`old_balance` and `new_balance` default to `0.0` when the request omits them, so a rule referencing them never errors on missing data. Send them under `metadata` to use real values.

### Custom fields and enrichment (`meta` / `enrichment`)

Beyond the fixed variables above, rules can read two open-ended maps with no engine change:

- **`meta`** — arbitrary request `metadata` (e.g. `country`, `mcc`, `device`).
- **`enrichment`** — externally-computed scores/flags supplied in the request `enrichment` object (e.g. `ml_score`, `sanctions_hit`, `ring_risk`). Osprey does not verify these; they are asserted by the caller's pipeline.

Because referencing an absent key errors at evaluation, **guard optional fields with `has()`**:

```cel
has(meta.country) && meta.country == "US"
has(enrichment.ml_score) && enrichment.ml_score > 0.9
has(enrichment.sanctions_hit) && enrichment.sanctions_hit
```

JSON numbers arrive as doubles, so compare enrichment numbers as doubles (`enrichment.ring_risk >= 3.0`). The live, authoritative variable list is served by `GET /rules/variables`.

## Rule Example

```json
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
```

Create it:

```bash
curl -fsS -X POST "$OSPREY_URL/rules" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json
```

Rules are active immediately after a successful write.

## Common Expressions

```cel
amount > 10000.0
amount >= 9000.0 && amount < 10000.0
debtor_id == creditor_id
old_balance > 0.0 && new_balance == 0.0
velocity_count > 5
tx_type == "CASH_OUT" || tx_type == "TRANSFER"
```

## Typology Checklist

Use a typology when several rules together describe a pattern.

| Question | Good Answer |
|----------|-------------|
| What pattern does it represent? | Structuring, mule activity, account takeover, rapid movement. |
| Which rules contribute? | Existing active rule IDs. |
| Are weights explainable? | Weights reflect relative signal strength. |
| What threshold alerts? | `alertThreshold` from `0` exclusive to `1` inclusive. |
| Is compliance mode enabled? | Typologies affect decisions only in compliance mode. |

## Typology Example

```json
{
  "id": "sandbox-verification-typology",
  "name": "Sandbox Verification Typology",
  "description": "Groups one sample same-party rule.",
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
curl -fsS -X POST "$OSPREY_URL/typologies" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/typology-same-party.json
```

The referenced `ruleId` must already be active.

## Test Before Promotion

At minimum, test:

| Test | Expected |
|------|----------|
| Normal transaction | `NALT` |
| Trigger transaction | `ALRT` or the expected score/reason change |
| Missing required field | `400` |
| Duplicate transaction ID | `409` |

Baseline verifier:

```bash
OSPREY_URL=https://your-osprey-host.example \
TENANT_ID=demo \
OSPREY_ADMIN_TOKEN=replace-with-admin-token \
./scripts/verify-sandbox.sh
```

A rule or typology is ready to use when:

- Its expression is explainable in one sentence.
- It has at least one passing and one triggering test transaction.
- The response reasons are understandable to an operator.
- It depends only on fields your integration sends.
- It appears in `GET /rules` or `GET /typologies` after creation.
