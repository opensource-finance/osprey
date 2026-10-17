# Example: Money-Mule Detection for Payments

This example shows a payments service that asks Osprey about each outbound payment before it sends the money.
It finds the money-mule pattern behind authorised push payment (APP) fraud.

In APP fraud, a criminal tricks a victim into a bank transfer.
The money goes to a mule account.
The mule account sends the money on quickly, often to several new payees, and then it is empty.
The victim payment looks normal on its own.
The mule account shows the pattern.

## Files

| File | What it contains |
|------|------------------|
| `rules.json` | The 5 rules that the service loads into Osprey. |
| `scenario.json` | 6 payments, each with the expected action. |
| `payment_guard.py` | The integration code. It loads the rules, sends each payment to Osprey, and maps the decision to an action. |
| `run.sh` | Starts a clean Osprey server, runs `payment_guard.py`, and stops the server. |

## Run It

You need Go 1.26+ and Python 3.
The example uses only the Python standard library.

From the repository root, run this command:

```bash
./examples/app-fraud-mule-detection/run.sh
```

The script builds Osprey and starts it in detection mode on port `18081`.
It uses a temporary SQLite database, so each run starts clean.
If a payment does not get the expected action, the script prints `FAIL` and exits with code `1`.

## How We Built It

### 1. Write the signals in plain language

| Signal | Why it matters | Data |
|--------|----------------|------|
| The sender account is new. | Criminals open or buy new accounts for mule activity. | `metadata.account_age_days`, from the core banking system |
| The payee is new. | A mule sends money to accounts that it did not pay before. | `metadata.new_payee`, from the payee list of the customer |
| Money goes in and out fast. | A mule forwards money in minutes, not days. | `velocity_count`, which Osprey calculates |
| The payment empties the account. | A mule does not keep a balance. | `metadata.old_balance` and `metadata.new_balance` |
| The payee name does not match. | A Confirmation of Payee (CoP) "no match" is a strong fraud signal. | `enrichment.cop_result`, from the CoP service |

Osprey does not know the age of an account or the CoP result.
The payments service already has these facts, so it sends them with each payment.
Facts about the customer go in `metadata`.
Results from other systems go in `enrichment`.

### 2. Write one rule for each signal

Each rule returns `true` or `false`.
Osprey converts `true` to a score of `1` and `false` to a score of `0`.
The `bands` of the rule convert the score to an outcome and a reason.

```json
{
  "id": "mule-pass-through",
  "expression": "velocity_count >= 3",
  "weight": 1,
  "bands": [
    {"lowerLimit": 1, "subRuleRef": ".review", "reason": "3 or more payments in or out of this account in the last hour"},
    {"lowerLimit": 0, "upperLimit": 1, "subRuleRef": ".pass", "reason": "Normal activity"}
  ]
}
```

The rules that read `meta` or `enrichment` use `has()` first.
Without `has()`, a payment without the field causes a rule error and an alert.

### 3. Choose between `.review` and `.fail`

The two outcomes have different effects in detection mode:

- **`.review`**: The rule adds its score to the weighted average. One weak signal alone does not cause an alert.
- **`.fail`**: The rule causes an alert, whatever the other rules return.

The 4 mule signals use `.review`, because each signal alone is normal for many honest customers.
The CoP "no match" rule uses `.fail`, because the bank does not want to send money to a name that does not match.

### 4. Know how Osprey calculates the score

In detection mode, the score is the weighted average of all active rule scores.
Osprey gives `ALRT` when the score is `0.7` or more, or when a rule gives `.fail`.
This example has 5 rules with weight `1`, so each signal adds `0.2`.
Thus, 4 of the 5 signals must be present for an alert from `.review` rules alone.

### 5. Map the decision to an action

The service uses the status and the reasons to select one of three actions:

```python
def check_payment(tx):
    result = call("POST", "/evaluate", tx)
    if result["status"] == "ALRT":
        return "hold", result     # stop the payment for a fraud analyst
    if result.get("reasons"):
        return "warn", result     # show a scam warning, then send if the customer confirms
    return "release", result      # send the payment
```

## Output

This is the output from a real run on the current `main` branch:

```text
loaded rule mule-new-account
loaded rule mule-new-payee
loaded rule mule-pass-through
loaded rule mule-balance-drain
loaded rule mule-cop-no-match

pay-001  cust-alice -> landlord-ltd  1200 GBP
  NALT  score 0.00  action RELEASE

pay-002  cust-carol -> ext-supplier  3400 GBP
  ALRT  score 0.40  action HOLD
  - Confirmation of Payee: the name does not match the account
  - First payment to this payee

pay-003  cust-bob -> cust-mule  2500 GBP
  NALT  score 0.20  action WARN
  - First payment to this payee

pay-004  cust-mule -> ext-x1  800 GBP
  NALT  score 0.40  action WARN
  - First payment to this payee
  - Sender account is less than 30 days old

pay-005  cust-mule -> ext-x2  800 GBP
  NALT  score 0.60  action WARN
  - 3 or more payments in or out of this account in the last hour
  - First payment to this payee
  - Sender account is less than 30 days old

pay-006  cust-mule -> ext-x3  850 GBP
  ALRT  score 0.80  action HOLD
  - 3 or more payments in or out of this account in the last hour
  - First payment to this payee
  - Payment empties more than 90% of the balance
  - Sender account is less than 30 days old

OK: all 6 payments got the expected action
```

## What the Output Shows

- **`pay-001`**: An established customer pays a known payee. No rule matches, so the money goes.
- **`pay-002`**: The score is only `0.40`, but the CoP rule gives `.fail`. Thus, the status is `ALRT`.
- **`pay-003`**: Bob, the scam victim, gets a warning. A good warning at this step can stop the scam.
- **`pay-004` and `pay-005`**: The mule account collects signals. The score goes up from `0.40` to `0.60`.
- **`pay-006`**: The last payment empties the account. The score is `0.80`, and the bank holds the money.

The bank holds 850 GBP of the 2,500 GBP.
To hold more of the money, tune the rules to alert earlier.
For example, give the new account rule a higher weight, or use `.fail` for a new account with 3 or more payments in an hour.

## Things to Know

- `velocity_count` counts the transactions in the last hour where the account is the debtor or the creditor. The count includes the current transaction. Thus, in `pay-005`, the count is 3: one payment in and two payments out.
- The velocity window uses the time when Osprey received the transaction, not the `timestamp` field.
- Rules apply to all tenants on the Osprey server.
- Each new rule changes the weighted average. If you add a rule, examine the score of each scenario again.
- Osprey does not verify `metadata` or `enrichment`. Your service is responsible for these values.
- The order of the `reasons` in the response can change. The example sorts them before it prints them.

## Adapt It

1. Replace the fields in `scenario.json` with the fields that your payments service has.
2. Change the rules in `rules.json` to match your risk signals.
3. Add one payment that must pass and one that must alert for each new rule.
4. Run `run.sh` until each payment gets the expected action.
5. Copy `check_payment()` into your service.
