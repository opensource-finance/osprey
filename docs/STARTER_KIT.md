# Starter Kit

Osprey includes example rules and typologies.
Public FATF guidance and the PaySim benchmark are the basis for these rules.
Use them only as a start.

**CAUTION:** Examine and tune the example rules before you use them in a live workflow.

## Load the FATF-Inspired Rules

1. Start Osprey with an admin token:

```bash
export OSPREY_ADMIN_TOKEN=local-admin-token
go run ./cmd/osprey
```

2. In a different terminal, load the rules:

```bash
./scripts/seed-starter-kit.sh
```

## Load Rules and Typologies

Osprey uses typologies only in compliance mode.

1. Start Osprey in compliance mode:

```bash
export OSPREY_ADMIN_TOKEN=local-admin-token
OSPREY_MODE=compliance go run ./cmd/osprey
```

2. In a different terminal, load the rules and the typologies:

```bash
./scripts/seed-starter-kit.sh --compliance
```

## FATF-Inspired Rules

File: `configs/rules/fatf-rules.json`

| Rule ID | Finds |
|---------|---------|
| `structuring-001` | Amounts just below the reporting thresholds. |
| `high-value-001` | Transactions of more than 10,000. |
| `very-high-value-001` | Transactions of more than 50,000. |
| `round-amount-001` | Amounts that are round to a suspicious degree. |
| `account-drain-001` | A balance that goes down to zero. |
| `partial-drain-001` | A balance decrease of more than 90%. |
| `same-party-001` | Transfers between the same party. |
| `velocity-001` | More than 5 recent transactions. |
| `velocity-extreme-001` | More than 10 recent transactions. |
| `high-risk-type-001` | `CASH_OUT` or `TRANSFER`. |
| `cash-intensive-001` | Transaction types that are similar to cash. |
| `micro-transaction-001` | Amounts of less than 10. |

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

To load the PaySim rules, run this command:

```bash
./scripts/seed-paysim.sh
```

We tuned these rules for the PaySim benchmark dataset.
We did not tune them for general production traffic.

## Test a Seeded Rule

To send a test transaction, run this command:

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

To create your own rule, run this command:

```bash
curl -fsS -X POST http://localhost:8080/rules \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo" \
  -H "Authorization: Bearer $OSPREY_ADMIN_TOKEN" \
  -d @docs/examples/rule-same-party.json
```

For the rule design checklist, refer to [Rule and typology authoring](RULE_TYPOLOGY_AUTHORING.md).
