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
| `pass-through-001` | A payment that sends on 80% to 120% of the money that the account received in the last hour. |
| `fan-out-001` | 5 or more different payees in the last hour. |
| `sanctions-001` | Debtor or creditor name matches the OFAC SDN list: `.fail` from 0.88, `.review` from 0.80. Weight `0.1`. Needs `OSPREY_SANCTIONS_DIR` and party `name` fields. |

All rules except `sanctions-001` use `.review`. Only a probable sanctions match causes an alert alone.

- Context rules (amount size, round amount, transaction type, cash, micro amounts) have weights from `0.05` to `0.1`.
- Behaviour rules (drain, structuring, same party, velocity, pass-through, fan-out) have weights from `0.2` to `0.4`.
- One behaviour rule plus all the context rules that can match together gives less than `0.7`.
  Thus, an alert needs two behaviour signals.
- A possible sanctions match from `sanctions-001` adds up to `0.088`.
  With it, one behaviour rule and the context rules can reach `0.7`.

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
| `typology-sanctions` | Probable OFAC SDN match. |

Each typology has the threshold `0.75`.
Its core rule has the weight `0.5`, and the other rules have `0.25` or `0.125`.
Thus, a typology triggers only when its core signal and one more signal are present.

## PaySim Rules

File: `configs/rules/paysim-rules.json`

To load the PaySim rules, run this command:

```bash
./scripts/seed-paysim.sh
```

We tuned these rules for the PaySim benchmark dataset.
We did not tune them for general production traffic.

## Measured Quality

We ran all 6,362,620 PaySim transactions through Osprey with `cmd/qualitybench`.
PaySim has 8,213 fraud transactions (0.13%).
Detection mode, sum scoring, threshold `0.7`:

| Rule set | Alerts | Alert rate | Fraud caught (recall) | Precision |
|----------|--------|------------|-----------------------|-----------|
| `paysim-rules.json` | 8,018 | 0.13% | 97.6% | 100% |
| `fatf-rules.json` | 12,656 | 0.20% | 0.3% | 0.2% |
| `fatf-rules.json` + typologies (compliance mode) | 12,119 | 0.19% | 0.2% | 0.1% |
| `paysim-rules.json` v1 (before this release) | 1,531,698 | 24.1% | 97.7% | 0.5% |
| `fatf-rules.json` v1 + typologies, compliance mode | 4,177,811 | 65.7% | 98.0% | 0.2% |

**CAUTION:** These numbers do not show quality on real traffic.

- PaySim is a simulator. Each PaySim fraud sends the whole balance of the account.
  `paysim-whole-balance` matches exactly that, so the PaySim rules get 100% precision.
  On real traffic, a whole-balance payment is a signal, not proof.
- Without `paysim-whole-balance`, the best rule that we found catches 31% of the fraud with 3.9% precision
  (`TRANSFER` or `CASH_OUT`, account drained, amount more than 1,000,000).
- The FATF rules look for money laundering, not for the account takeover that PaySim simulates.
  Their main signals use velocity, and PaySim accounts almost never send more than one payment.
  A fair test of the FATF rules needs an AML dataset, for example SAML-D or IBM AMLworld.

To measure a rule set on PaySim yourself, run this command:

```bash
go run ./cmd/qualitybench -csv PS_20174392719_1491204439457_log.csv -rules configs/rules/paysim-rules.json
```

Add `-typologies configs/typologies/fatf-typologies.json` for compliance mode.
Add `-velocity` to replay the rows in order with velocity windows on PaySim time.
With `-velocity`, the run is slower, because each row goes into a SQLite store.

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
