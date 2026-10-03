# Security Policy

Osprey is transaction-monitoring infrastructure for fraud and AML/CFT workflows.
We take security reports seriously. We thank researchers for responsible disclosure.

## Supported Versions

| Version | Supported |
|---------|-----------|
| `0.1.x` (latest minor) | ✅ |
| `< 0.1.0` / unreleased `main` | ❌ (no formal support. Fixes go to `main`.) |

Until `1.0.0`, only the latest tagged minor version gets security fixes.

## Reporting a Vulnerability

**WARNING:** Do not open a public issue for a security problem. A public issue shows the problem to attackers before a fix is available.

Report the problem privately through GitHub:

1. Go to the repository's **Security** tab → **Report a vulnerability**
   (or open <https://github.com/opensource-finance/osprey/security/advisories/new>).
2. GitHub creates a private security advisory. Only you and the maintainers can see it.

If possible, include this information:

- A description of the issue and its impact.
- The steps to reproduce the issue. A minimal request or rule that triggers it is ideal.
- The affected version or commit (`osprey --version` / the `version` field on `GET /health`).
- A possible fix, if you have one.

### What to expect

- **Acknowledgement** within 3 business days.
- An initial assessment (severity + whether we can reproduce) within 7 business days.
- Coordinated disclosure:
  1. We agree on a timeline with you.
  2. We ship a fix.
  3. We credit you in the advisory and release notes. If you prefer, you can stay anonymous.

## Scope and Trust Model

Some behaviors are **by design**. They are not vulnerabilities.
Read these trust boundaries before you send a report:

- **The admin token is fully trusted.** A person with `OSPREY_ADMIN_TOKEN` can
  write, change, and delete rules and typologies (see [docs/SANDBOX.md](docs/SANDBOX.md)).
  - **WARNING:** Protect the token like a production credential. A person with the token can change all rules.
  - A weak token is a configuration issue, not a vulnerability. An example report is "I set a weak token and someone changed my rules".
- **The caller asserts the `enrichment` request field.** The caller computes these signals outside Osprey, for example `ml_score` and `sanctions_hit`.
  - Osprey trusts these values as supplied. Osprey does **not** verify them.
  - The caller's ingestion pipeline is the trust boundary.
  - [docs/RULE_TYPOLOGY_AUTHORING.md](docs/RULE_TYPOLOGY_AUTHORING.md) gives more information.
- **Users write the rules in CEL.** Expressions run in the Google CEL sandbox (no I/O, no unbounded loops).
  - A bad rule can still give wrong decisions.
  - Examine the rules before you use them in a live workflow.

These problems are in scope. Report them:

- Auth bypass on protected endpoints.
- Tenant isolation breaks. For example, one tenant reads or changes the data or rules of a different tenant.
- CEL sandbox escapes.
- Injection.
- Denial of service from untrusted input.
- Secret leakage in logs or errors.

## Safe Harbor

We will not start or support legal action against a researcher who obeys these conditions:

- The researcher acts in good faith.
- The researcher does not violate privacy or degrade the service.
- The researcher gives us a reasonable time to fix the problem before public disclosure.
