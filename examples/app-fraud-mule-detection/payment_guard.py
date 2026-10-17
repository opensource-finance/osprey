#!/usr/bin/env python3
"""Payment guard: ask Osprey about each outbound payment before you release it.

This is the integration code. A payments service calls check_payment() before it
sends money and maps the Osprey decision to one of three actions:

  release  NALT and no reasons      -> send the payment
  warn     NALT with review reasons -> show a scam warning, send if the customer confirms
  hold     ALRT                     -> stop the payment for a fraud analyst

Standard library only. Run it through run.sh, which starts a clean Osprey server.
"""

import json
import os
import sys
import urllib.error
import urllib.request

OSPREY_URL = os.environ.get("OSPREY_URL", "http://localhost:8080")
ADMIN_TOKEN = os.environ.get("OSPREY_ADMIN_TOKEN", "")
TENANT = os.environ.get("TENANT_ID", "neobank-uk")
HERE = os.path.dirname(os.path.abspath(__file__))


def call(method, path, body=None, admin=False):
    headers = {"Content-Type": "application/json", "X-Tenant-ID": TENANT}
    if admin:
        headers["Authorization"] = "Bearer " + ADMIN_TOKEN
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(OSPREY_URL + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.load(resp)
    except urllib.error.HTTPError as err:
        raise SystemExit(f"{method} {path} -> {err.code}: {err.read().decode()}")


def load_rules():
    with open(os.path.join(HERE, "rules.json")) as f:
        for rule in json.load(f):
            call("POST", "/rules", rule, admin=True)
            print(f"loaded rule {rule['id']}")


def check_payment(tx):
    """Return (action, osprey_response) for one outbound payment."""
    result = call("POST", "/evaluate", tx)
    if result["status"] == "ALRT":
        return "hold", result
    if result.get("reasons"):
        return "warn", result
    return "release", result


def main():
    load_rules()
    with open(os.path.join(HERE, "scenario.json")) as f:
        scenario = json.load(f)

    failures = 0
    for step in scenario:
        tx = step["tx"]
        action, res = check_payment(tx)
        ok = action == step["expect"]
        failures += not ok
        print()
        print(
            f"{tx['id']}  {tx['debtor']['id']} -> {tx['creditor']['id']}  "
            f"{tx['amount']['value']} {tx['amount']['currency']}"
        )
        print(
            f"  {res['status']}  score {res['score']:.2f}  action {action.upper()}"
            + ("" if ok else f"   <-- expected {step['expect'].upper()}")
        )
        for reason in sorted(res.get("reasons", [])):
            print(f"  - {reason}")

    print()
    if failures:
        print(f"FAIL: {failures} payment(s) did not get the expected action")
        sys.exit(1)
    print(f"OK: all {len(scenario)} payments got the expected action")


if __name__ == "__main__":
    main()
