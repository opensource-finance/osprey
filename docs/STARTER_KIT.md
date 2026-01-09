# Osprey Starter Kit

Osprey includes pre-built rules and typologies based on public FATF (Financial Action Task Force) guidance. This enables quick deployment with production-ready AML/CFT detection capabilities.

## Quick Start

```bash
# Start Osprey
go run ./cmd/osprey

# Load FATF-aligned rules (Detection mode)
./scripts/seed-starter-kit.sh

# Or load with typologies (Compliance mode)
OSPREY_MODE=compliance go run ./cmd/osprey &
./scripts/seed-starter-kit.sh --compliance
```

## Available Rule Sets

### 1. FATF Rules (`configs/rules/fatf-rules.json`)

Production-ready rules based on FATF recommendations:

| Rule ID | Name | Description | Weight |
|---------|------|-------------|--------|
| `structuring-001` | Structuring Detection | Amounts just below reporting thresholds | 0.6 |
| `high-value-001` | High Value Transaction | Transactions > $10,000 | 0.3 |
| `very-high-value-001` | Very High Value | Transactions > $50,000 | 0.5 |
| `round-amount-001` | Round Amount | Suspiciously round numbers | 0.2 |
| `account-drain-001` | Account Drain | Balance drained to zero | 0.8 |
| `partial-drain-001` | Partial Drain | >90% balance reduction | 0.5 |
| `same-party-001` | Same Party Transfer | Self-transfers (layering) | 1.0 |
| `velocity-001` | High Velocity | >5 transactions in window | 0.6 |
| `velocity-extreme-001` | Extreme Velocity | >10 transactions in window | 0.8 |
| `high-risk-type-001` | High Risk Type | CASH_OUT/TRANSFER types | 0.2 |
| `cash-intensive-001` | Cash Intensive | Cash-based transactions | 0.3 |
| `micro-transaction-001` | Micro Transaction | Amount < $10 (card testing) | 0.3 |

### 2. PaySim Rules (`configs/rules/paysim-rules.json`)

Rules optimized for the PaySim benchmark dataset (~96% recall):

| Rule ID | Name | Weight |
|---------|------|--------|
| `paysim-account-drain` | Account drained to zero | 0.8 |
| `paysim-high-risk-type` | CASH_OUT or TRANSFER | 0.3 |
| `paysim-fraud-pattern` | Combined drain + type | 1.0 |
| `paysim-large-amount` | Amount > $200K | 0.4 |
| `paysim-partial-drain` | >90% drain + high amount | 0.7 |

## Available Typologies

Typologies are for **Compliance mode** only. They combine multiple rules to detect complex patterns.

### FATF Typologies (`configs/typologies/fatf-typologies.json`)

| Typology ID | Name | Threshold | Rules |
|-------------|------|-----------|-------|
| `typology-structuring` | Structuring (Smurfing) | 0.5 | structuring + round-amount + velocity |
| `typology-account-takeover` | Account Takeover | 0.6 | account-drain + risk-type + velocity + high-value |
| `typology-mule-account` | Mule Account Activity | 0.55 | velocity + high-value + partial-drain + risk-type |
| `typology-rapid-movement` | Rapid Movement of Funds | 0.5 | extreme-velocity + velocity + risk-type |
| `typology-cash-intensive` | Cash Intensive Business | 0.5 | cash-intensive + high-value + round-amount + velocity |
| `typology-fraud-basic` | Basic Fraud Detection | 0.6 | account-drain + same-party + high-value + velocity |

## Sources

All rules and typologies are based on publicly available guidance:

- **FATF Recommendations**: https://www.fatf-gafi.org/en/publications/Fatfrecommendations.html
- **FATF Methods and Trends**: https://www.fatf-gafi.org/en/topics/methods-and-trends.html
- **Trade-Based ML Indicators**: https://www.fatf-gafi.org/en/publications/Methodsandtrends/Trade-based-money-laundering-indicators.html
- **PaySim Dataset**: Lopez-Rojas et al., "PaySim: A financial mobile money simulator for fraud detection" (2016)

## Usage Examples

### Detection Mode (Default)

```bash
# Start server
go run ./cmd/osprey

# Load FATF rules
./scripts/seed-starter-kit.sh

# Test a transaction
curl -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: default" \
  -d '{
    "type": "TRANSFER",
    "debtor": {"id": "user1", "accountId": "acc1"},
    "creditor": {"id": "user2", "accountId": "acc2"},
    "amount": {"value": 9500, "currency": "USD"}
  }'

# Response: score based on weighted rule results
```

### Compliance Mode

```bash
# Start server in Compliance mode
OSPREY_MODE=compliance go run ./cmd/osprey

# Load FATF rules AND typologies
./scripts/seed-starter-kit.sh --compliance

# Test - response includes typology evaluation
curl -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: default" \
  -d '{
    "type": "CASH_OUT",
    "debtor": {"id": "user1", "accountId": "acc1"},
    "creditor": {"id": "user2", "accountId": "acc2"},
    "amount": {"value": 9500, "currency": "USD"},
    "metadata": {"old_balance": 10000, "new_balance": 500}
  }'
```

### PaySim Benchmark

```bash
# Load PaySim-optimized rules
./scripts/seed-paysim.sh

# Run benchmark
./benchmark -csv data/paysim.csv -limit 50000

# Expected results:
#   Precision: ~100%
#   Recall: ~96%
#   F1-Score: ~0.98
```

## Customization

### Adding Custom Rules

```bash
curl -X POST http://localhost:8080/rules \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: default" \
  -d '{
    "id": "my-custom-rule",
    "name": "Custom Rule",
    "description": "My custom detection logic",
    "expression": "amount > 25000.0 && tx_type == \"WIRE\"",
    "weight": 0.5,
    "enabled": true,
    "bands": [
      {"lowerLimit": 1.0, "subRuleRef": ".review", "reason": "Custom condition met"},
      {"lowerLimit": 0.0, "upperLimit": 1.0, "subRuleRef": ".pass", "reason": "Normal"}
    ]
  }'

# Reload to apply
curl -X POST http://localhost:8080/rules/reload -H "X-Tenant-ID: default"
```

### CEL Expression Reference

Rules use [Google CEL](https://github.com/google/cel-go) expressions. Available variables:

| Variable | Type | Description |
|----------|------|-------------|
| `amount` | double | Transaction amount |
| `currency` | string | Currency code |
| `tx_type` | string | Transaction type |
| `debtor_id` | string | Sender ID |
| `creditor_id` | string | Receiver ID |
| `old_balance` | double | Pre-transaction balance |
| `new_balance` | double | Post-transaction balance |
| `velocity_count` | int | Recent transaction count |

### Expression Examples

```cel
// High value
amount > 10000.0

// Structuring (just below threshold)
amount >= 9000.0 && amount < 10000.0

// Account drain
old_balance > 0.0 && new_balance == 0.0

// Round amounts
amount >= 1000.0 && amount == double(int(amount / 1000.0)) * 1000.0

// Same party
debtor_id == creditor_id

// High risk type
tx_type == "CASH_OUT" || tx_type == "TRANSFER"

// Velocity check
velocity_count > 5

// Combined conditions
(amount > 5000.0) && (tx_type == "TRANSFER") && (velocity_count > 3)
```

## Regulatory Alignment

| Regulation | Covered By |
|------------|------------|
| FATF Rec 10 (CDD) | High value rules, velocity checks |
| FATF Rec 19 (Higher-risk countries) | Can add geographic rules |
| FATF Rec 20 (Suspicious transactions) | All typologies |
| FATF Rec 22 (DNFBPs) | Cash intensive rules |
| BSA/AML (US) | Structuring, high value rules |
| 4AMLD/5AMLD (EU) | All FATF-aligned rules |

## Performance

With the starter kit loaded:

| Metric | Value |
|--------|-------|
| Rules evaluated | 12 (FATF set) |
| Evaluation latency | <5ms |
| Throughput | >4,000 TPS |
| Memory overhead | ~10MB |
