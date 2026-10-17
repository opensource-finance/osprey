# Examples

Each example is a small project that uses Osprey for one real use case.
Each example includes the rules, a scenario, the integration code, and a script that runs the scenario against a real Osprey server.
The script checks each decision, so the output in each walkthrough is the real output.

| Example | Mode | What it shows |
|---------|------|---------------|
| [Money-mule detection for payments](app-fraud-mule-detection/README.md) | `detection` | Rule weights, `velocity_count`, balance fields, `.review` and `.fail` outcomes, and a release, warn, or hold action. |
| [Structuring detection for a money transfer service](remittance-structuring/README.md) | `compliance` | Typologies, `velocity_amount_sum`, enrichment from a sanctions system, and a case file from `GET /evaluations/{id}`. |

## Run All Examples

You need Go 1.26+, Python 3, and curl.
To run all examples, run this command from the repository root:

```bash
make examples
```

Each example starts its own Osprey server on a different port with a temporary database.
If an example does not get the expected decisions, the command fails.
