# Starter Kit

Osprey ships with example rules and typologies inspired by public FATF guidance and the PaySim benchmark. Treat them as starting points. Review and tune them before using them in a live workflow.

## Load the FATF-Inspired Rules

```bash
export OSPREY_ADMIN_TOKEN=local-admin-token
go run ./cmd/osprey
```

In another terminal:

```bash
./scripts/seed-starter-kit.sh
```

## Load Rules and Typologies

Typologies are used only in compliance mode.

```bash
export OSPREY_ADMIN_TOKEN=local-admin-token
OSPREY_MODE=compliance go run ./cmd/osprey
```

In another terminal:

```bash
./scripts/seed-starter-kit.sh --compliance
```

## FATF-Inspired Rules

File: `configs/rules/fatf-rules.json`

| Rule ID | Detects |
|---------|---------|
| `structuring-001` | Amounts just below reporting thresholds. |
| `high-value-001` | Transactions over 10,000. |
| `very-high-value-001` | Transactions over 50,000. |
| `round-amount-001` | Suspiciously round amounts. |
| `account-drain-001` | Balance drained to zero. |
| `partial-drain-001` | More than 90% balance reduction. |
| `same-party-001` | Same-party transfers. |
| `velocity-001` | More than 5 recent transactions. |
| `velocity-extreme-001` | More than 10 recent transactions. |
| `high-risk-type-001` | `CASH_OUT` or `TRANSFER`. |
| `cash-intensive-001` | Cash-like transaction types. |
| `micro-transaction-001` | Amounts below 10. |

## FATF-Inspired Typologies

File: `configs/typologies/fatf-typologies.json`

| Typology ID | Pattern |
|-------------|---------|
| `typology-structuring` | Structuring. |
| `typology-account-takeover` | Account takeover. |
| `typology-mule-account` | Mule-account activity. |
| `typology-rapid-movement` | Rapid movement of funds. |
| `typology-cash-intensive` | Cash-intensive activity. |
| `typology-fraud-basic` | Basic fraud pattern. |

## PaySim Rules

File: `configs/rules/paysim-rules.json`

```bash
./scripts/seed-paysim.sh
```

These rules are tuned for the PaySim benchmark dataset, not general production traffic.

## Test a Seeded Rule

```bash
curl -fsS -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -d '{
    "id": "starter-tx-001",
    "type": "TRANSFER",
    "debtor": {"id": "user1", "accountId": "acc1"},
    "creditor": {"id": "user2", "accountId": "acc2"},
    "amount": {"value": 9500, "currency": "USD"},
    "timestamp": "2026-05-25T09:15:30Z"
  }'
```

## Customize

Create your own rules with:

```bash
curl -fsS -X POST http://localhost:8080/rules \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json
```

See [Rule and typology authoring](RULE_TYPOLOGY_AUTHORING.md) for the rule design checklist.
