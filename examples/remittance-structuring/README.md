# Example: Structuring Detection for a Money Transfer Service

This example shows a money transfer service that screens each transfer with Osprey in compliance mode.
It finds structuring, and it finds transfers to parties on a sanctions list.

Many reporting rules start at 10,000.
An example is the US Currency Transaction Report for cash.
Some customers split one large amount into several transfers just below the threshold.
This is structuring.
Each transfer looks normal on its own.
The pattern shows only across the transfers.

## Files

| File | What it contains |
|------|------------------|
| `rules.json` | The 4 rules. Each rule finds one signal. |
| `typologies.json` | The 2 typologies. Each typology groups rules into one pattern. |
| `scenario.json` | 6 transfers, each with the expected decision. |
| `remittance_monitor.py` | The integration code. It loads the configuration, screens each transfer, and writes a case file for each alert. |
| `run.sh` | Starts a clean Osprey server, runs `remittance_monitor.py`, and stops the server. |

## Run It

You need Go 1.26+ and Python 3.
The example uses only the Python standard library.

From the repository root, run this command:

```bash
./examples/remittance-structuring/run.sh
```

The script builds Osprey and starts it in compliance mode on port `18082`.
It uses a temporary SQLite database, so each run starts clean.
If a transfer does not get the expected decision, the script prints `FAIL` and exits with code `1`.

## How We Built It

### 1. Write one rule for each signal

| Rule | Expression | Signal |
|------|------------|--------|
| `rem-near-threshold` | `amount >= 9000.0 && amount < 10000.0` | The amount is just below the threshold. |
| `rem-repeat-sender` | `velocity_count >= 3` | The customer sent 3 or more transfers in the last hour. |
| `rem-hour-total` | `velocity_amount_sum >= 25000.0` | The customer moved 25,000 or more in the last hour. |
| `rem-sanctions-hit` | `has(enrichment.sanctions_hit) && enrichment.sanctions_hit == true` | The screening system matched the recipient to a sanctions list. |

All 4 rules use the `.review` outcome.
In compliance mode, the typologies make the decision.
A `.fail` outcome causes an alert without the typologies, so this example does not use it.

Osprey calculates `velocity_count` and `velocity_amount_sum` from the transactions that it stored.
The caller does not send them.
The sanctions result comes from a different system, so the caller sends it in `enrichment`.

### 2. Group the rules into typologies

A typology adds the rule scores, each multiplied by its weight.
If the total is equal to or more than `alertThreshold`, the typology triggers.

```json
{
  "id": "rem-typology-structuring",
  "alertThreshold": 0.75,
  "rules": [
    {"ruleId": "rem-near-threshold", "weight": 0.5},
    {"ruleId": "rem-repeat-sender", "weight": 0.25},
    {"ruleId": "rem-hour-total", "weight": 0.25}
  ]
}
```

With these weights, an amount near the threshold is not enough for an alert.
The amount must come with a second signal.
A large amount with high frequency, but not near the threshold, gives only `0.5`.

The sanctions typology has one rule with weight `1.0` and a threshold of `1.0`.
Thus, one sanctions match is enough for an alert.

Load the rules before the typologies.
Osprey rejects a typology that refers to a rule that is not active.

### 3. Write a case file for each alert

The `/evaluate` response gives the status, the score, and the reasons.
The stored evaluation also gives the contribution of each rule to each typology.
The monitor reads it from `GET /evaluations/{id}` for each alert:

```python
def case_file(evaluation_id):
    evaluation = call("GET", f"/evaluations/{evaluation_id}")
    for t in evaluation["typologyResults"]:
        if t["triggered"]:
            ...  # t["score"], t["threshold"], and t["contributions"]
```

An analyst can see why the transfer caused an alert without a new calculation.

## Output

This is the output from a real run on the current `main` branch:

```text
loaded rule rem-near-threshold
loaded rule rem-repeat-sender
loaded rule rem-hour-total
loaded rule rem-sanctions-hit
loaded typology rem-typology-structuring
loaded typology rem-typology-sanctions

rem-001  cust-eve -> rcp-eve-family  450 USD
  NALT  score 0.00

rem-002  cust-frank -> rcp-notary  15000 USD
  NALT  score 0.00

rem-003  cust-dana -> rcp-dana-1  9500 USD
  NALT  score 0.50
  - Amount is just below the 10,000 reporting threshold

rem-004  cust-dana -> rcp-dana-1  9400 USD
  NALT  score 0.50
  - Amount is just below the 10,000 reporting threshold

rem-005  cust-dana -> rcp-dana-1  9700 USD
  ALRT  score 1.00
  - 25,000 or more moved by this customer in the last hour
  - 3 or more transfers by this customer in the last hour
  - Amount is just below the 10,000 reporting threshold
  case file:
    typology rem-typology-structuring: score 1.00 >= threshold 0.75
      rem-near-threshold   rule score 1 x weight 0.50 = 0.50
      rem-repeat-sender    rule score 1 x weight 0.25 = 0.25
      rem-hour-total       rule score 1 x weight 0.25 = 0.25

rem-006  cust-gus -> rcp-flagged  200 USD
  ALRT  score 1.00
  - Recipient matches a sanctions list entry
  case file:
    typology rem-typology-sanctions: score 1.00 >= threshold 1.00
      rem-sanctions-hit    rule score 1 x weight 1.00 = 1.00

OK: all 6 transfers got the expected decision
```

## What the Output Shows

- **`rem-001`**: A small family transfer. No rule matches.
- **`rem-002`**: A single transfer of 15,000. It is large, but it is not structuring. No rule matches.
- **`rem-003` and `rem-004`**: Each amount is just below the threshold. The structuring score is `0.50`, which is less than `0.75`.
- **`rem-005`**: The third transfer completes the pattern. Dana sent 28,600 in 3 transfers in one hour. All 3 rules match, and the score is `1.00`.
- **`rem-006`**: The amount is small, but the recipient matches a sanctions list. The sanctions typology triggers alone.

In compliance mode, the `score` in the response is the highest typology score.
The reasons come from all rules that gave `.review`, `.fail`, or `.err`.

## Things to Know

- Compliance mode needs a minimum of one typology. Without typologies, `POST /evaluate` returns `503`.
- `velocity_count` and `velocity_amount_sum` include the current transaction. They also include transactions where the customer is the creditor.
- The velocity window is one hour. It uses the time when Osprey received the transaction, not the `timestamp` field.
- Rules and typologies apply to all tenants on the Osprey server.
- The weights in this example are `0.5` and `0.25`. These values are exact in binary floating point, so a sum of `0.75` is exactly `0.75`.
- Osprey does not verify `enrichment`. Your sanctions screening system is responsible for `sanctions_hit`.

## Adapt It

1. Set the threshold in `rem-near-threshold` to the reporting threshold that applies to you.
2. Change the frequency and the hourly total to match your customers.
3. Add one transfer that must pass and one that must alert for each change.
4. Run `run.sh` until each transfer gets the expected decision.
5. Copy `screen_transfer()` and `case_file()` into your service.
