# Osprey Integration Tests

## Quick Start

```bash
# 1. Start Osprey server (in another terminal)
cd /path/to/osprey
go run cmd/osprey/main.go

# 2. Run tests
cd tests/integration
go test -v ./...
```

## Evaluation Modes

Osprey supports two evaluation modes. Tests run against whichever mode the server is started in:

| Mode | Command | Behavior |
|------|---------|----------|
| **Detection** (default) | `./osprey` | Rules only, no typology evaluation |
| **Compliance** | `OSPREY_MODE=compliance ./osprey` | Rules + Typologies |

### Verify Current Mode

```bash
curl http://localhost:8080/health | jq .mode
# Returns: "detection" or "compliance"
```

## Understanding the Domain

### What is Osprey?

Osprey is a **transaction monitoring engine** for Anti-Money Laundering (AML). It evaluates financial transactions against rules to detect suspicious patterns.

### Key Concepts

| Concept | What It Is | Example |
|---------|------------|---------|
| **Transaction** | A money transfer between parties | Alice sends $500 to Bob |
| **Rule** | A fraud detection pattern | "Flag amounts > $10,000" |
| **Band** | Maps numeric scores to outcomes | Score 0.5+ = "review" |
| **Typology** | Groups related rules | "Structuring detection" |
| **Evaluation** | Final verdict | "ALRT" or "NALT" |

### How Evaluation Works

**Detection Mode (Default):**
```
Transaction → Rules → Weighted Score → Threshold → ALRT or NALT
```

**Compliance Mode:**
```
Transaction → Rules → Typologies → FATF Patterns → ALRT or NALT
```

1. **Transaction arrives** with debtor, creditor, amount
2. **Each rule evaluates** the transaction using CEL expressions
3. **Scores map to bands** (.pass, .review, .fail)
4. **Mode determines next step:**
   - Detection: Weighted aggregate of rule scores
   - Compliance: Typology engine evaluates FATF patterns
5. **Final decision**: If any .fail OR score ≥ threshold → ALRT

### Built-in Rules

| Rule | Expression | Triggers When |
|------|------------|---------------|
| `high-value-001` | `amount > 10000 ? 1.0 : 0.0` | Amount exceeds $10,000 |
| `velocity-check-001` | `velocity_count > 10 ? 1.0 : ...` | Too many recent transactions |
| `same-account-001` | `debtor_id == creditor_id ? 1.0 : 0.0` | Sending to yourself |

### Band Configuration

Each rule has bands that convert scores to outcomes:

```
Score Range    Outcome    Meaning
-----------    -------    -------
0.0 - 0.5      .pass      Transaction is okay
0.5 - 1.0      .review    Needs human review
1.0+           .fail      Critical - block/alert
```

## Test Scenarios

| Test | What It Verifies |
|------|------------------|
| `TestNormalTransaction_NoAlert` | Small transfers pass |
| `TestHighValueTransaction_Alert` | Large transfers alert |
| `TestExactThreshold_NoAlert` | Boundary at $10,000 |
| `TestJustAboveThreshold_Alert` | $10,000.01 triggers |
| `TestSameAccountTransfer_Alert` | Self-transfers detected |
| `TestMultipleRulesTriggered_HighScore` | Compound risk |
| `TestMissingDebtorID_Error` | Input validation |
| `TestMissingTenantHeader_Error` | Multi-tenancy enforced |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OSPREY_TEST_URL` | `http://localhost:8080` | Osprey API URL |

## Troubleshooting

### Tests fail with "connection refused"
Osprey server is not running. Start it first.

### Tests fail with 401 Unauthorized
X-Tenant-ID header is missing. Check the test is sending headers.

### Tests fail with unexpected status
Check if rules are loaded. Hit `GET /rules` to verify.
