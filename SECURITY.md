# Security Policy

Osprey is transaction-monitoring infrastructure used in fraud and AML/CFT
workflows. We take security reports seriously and appreciate responsible
disclosure.

## Supported Versions

| Version | Supported |
|---------|-----------|
| `0.1.x` (latest minor) | ✅ |
| `< 0.1.0` / unreleased `main` | ❌ (no formal support; fixes land on `main`) |

Until `1.0.0`, only the latest tagged minor receives security fixes.

## Reporting a Vulnerability

**Please do not open a public issue for security problems.**

Report privately through GitHub:

1. Go to the repository's **Security** tab → **Report a vulnerability**
   (or open <https://github.com/opensource-finance/osprey/security/advisories/new>).
2. This creates a private security advisory visible only to you and the maintainers.

Include, where possible:

- A description of the issue and its impact.
- Steps to reproduce (a minimal request/rule that triggers it is ideal).
- Affected version or commit (`osprey --version` / the `version` field on `GET /health`).
- Any suggested remediation.

### What to expect

- **Acknowledgement** within 3 business days.
- An initial assessment (severity + whether we can reproduce) within 7 business days.
- Coordinated disclosure: we agree on a timeline with you, ship a fix, and credit
  you in the advisory and release notes unless you prefer to remain anonymous.

## Scope and Trust Model

Some behaviors are **by design**, not vulnerabilities. Understanding the trust
boundaries avoids false reports:

- **The admin token is fully trusted.** Anyone holding `OSPREY_ADMIN_TOKEN` can
  author, modify, and delete rules and typologies (see [docs/SANDBOX.md](docs/SANDBOX.md)).
  Protect the token like a production credential. "I set a weak token and someone
  changed my rules" is a configuration issue, not a vulnerability.
- **The `enrichment` request field is caller-asserted.** Externally computed
  signals passed in `enrichment` (e.g. `ml_score`, `sanctions_hit`) are trusted as
  supplied; Osprey does **not** independently verify them. The caller's ingestion
  pipeline is the trust boundary. This is documented in
  [docs/RULE_TYPOLOGY_AUTHORING.md](docs/RULE_TYPOLOGY_AUTHORING.md).
- **Rules are user-authored CEL.** Expressions run in Google's sandboxed CEL
  environment (no I/O, no unbounded loops), but a badly written rule can still
  produce wrong decisions. Review rules before using them in a live workflow.

In scope and worth reporting: auth bypass on protected endpoints, tenant isolation
breaks (one tenant reading/altering another's data or rules), CEL sandbox escapes,
injection, denial of service from untrusted input, or secret leakage in logs/errors.

## Safe Harbor

We will not pursue or support legal action against researchers who act in good
faith, avoid privacy violations and service degradation, and give us reasonable
time to remediate before public disclosure.
