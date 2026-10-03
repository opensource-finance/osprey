# Rule and Typology Authoring

Write each Osprey rule so that it is small, easy to explain, and easy to test.
A good rule finds one risk signal.

## Authoring Flow

1. Write the risk signal in plain language.
2. Find the fields in the `/evaluate` request that show the signal.
3. Write one CEL expression.
4. Add clear review reasons in `bands`.
5. Test one transaction that must give `NALT`.
6. Test one transaction that must give `ALRT`.
7. If a pattern needs more than one signal, put the rules into a typology.

## Rule Checklist

| Question | Good Answer |
|----------|-------------|
| What risk does this rule find? | One specific signal. |
| Which fields does it use? | Fields from the transaction or `metadata`. |
| What starts a review? | A precise CEL expression. |
| How strong is it? | A `weight` from `0` to `1`. |
| What does the operator see? | A short reason. |

Use many simple rules, not one expression that is difficult to understand.

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
| `velocity_count` | int | Count of recent transactions from the debtor in the window |
| `velocity_amount_sum` | double | Sum of the recent transaction amounts from the debtor in the window |
| `velocity_distinct_creditors` | int | Count of different counterparties of the debtor in the window |

If the request does not include `old_balance` or `new_balance`, the value is `0.0`.
Thus, a rule that uses these variables does not cause an error if the request does not include them.
To use real values, send them in `metadata`.

### Custom fields and enrichment (`meta` / `enrichment`)

Rules can also read two open maps.
You do not have to change the engine to use them.

- **`meta`**: any field in the request `metadata`. Examples are `country`, `mcc`, and `device`.
- **`enrichment`**: scores and flags from an external system, in the request `enrichment` object. Examples are `ml_score`, `sanctions_hit`, and `ring_risk`. Osprey does not verify these values. The pipeline of the caller is responsible for them.

**WARNING:** Use `has()` before you read an optional field in `meta` or `enrichment`. If the map does not contain the key, the expression has an error at evaluation.

A rule with an evaluation error gives a `.err` result.
A `.err` result changes the decision to `ALRT` and puts the error in `reasons`.
The other rules continue to give scores.

Examples of guarded fields:

```cel
has(meta.country) && meta.country == "US"
has(enrichment.ml_score) && enrichment.ml_score > 0.9
has(enrichment.sanctions_hit) && enrichment.sanctions_hit
```

Osprey reads JSON numbers as doubles.
Thus, compare enrichment numbers as doubles, for example `enrichment.ring_risk >= 3.0`.
`GET /rules/variables` gives the current, official list of variables.

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

To create the rule, run this command:

```bash
curl -fsS -X POST "$OSPREY_URL/rules" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json
```

A rule becomes active immediately after a successful write.

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

Use a typology when a set of rules together shows a pattern.

| Question | Good Answer |
|----------|-------------|
| Which pattern does it show? | Structuring, mule activity, account takeover, or rapid movement. |
| Which rules add to it? | The IDs of rules that exist and are active. |
| Can you explain the weights? | Each weight shows the relative strength of its signal. |
| Which threshold gives an alert? | `alertThreshold`, more than `0` and not more than `1`. |
| Is compliance mode on? | Typologies change decisions only in compliance mode. |

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

To create the typology, run this command:

```bash
curl -fsS -X POST "$OSPREY_URL/typologies" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/typology-same-party.json
```

The rule in `ruleId` must be active before you create the typology.

## Test Before Promotion

Do a minimum of these tests:

| Test | Expected |
|------|----------|
| Normal transaction | `NALT` |
| Transaction that starts the rule | `ALRT`, or the expected change to the score or the reason |
| Request without a required field | `400` |
| Duplicate transaction ID | `409` |

Run the baseline verifier:

```bash
OSPREY_URL=https://your-osprey-host.example \
TENANT_ID=demo \
OSPREY_ADMIN_TOKEN=replace-with-admin-token \
./scripts/verify-sandbox.sh
```

A rule or a typology is ready for use when all of these conditions are true:

- You can explain its expression in one sentence.
- It has a minimum of one test transaction that passes and one test transaction that starts the rule.
- An operator can understand the reasons in the response.
- It uses only the fields that your integration sends.
- It shows in `GET /rules` or `GET /typologies` after you create it.
