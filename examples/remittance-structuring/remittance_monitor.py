#!/usr/bin/env python3
"""Remittance monitor: screen each transfer with Osprey in compliance mode.

This is the integration code. A money transfer service calls screen_transfer()
for each transfer. On ALRT, it reads the stored evaluation from Osprey and
writes a case file that shows which typology triggered and why.

Standard library only. Run it through run.sh, which starts a clean Osprey server.
"""

import json
import os
import sys
import urllib.error
import urllib.request

OSPREY_URL = os.environ.get("OSPREY_URL", "http://localhost:8080")
ADMIN_TOKEN = os.environ.get("OSPREY_ADMIN_TOKEN", "")
TENANT = os.environ.get("TENANT_ID", "remit-us")
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


def load_config():
    # Rules first: a typology can only refer to rules that are already active.
    for kind, name, path in (("rule", "rules.json", "/rules"), ("typology", "typologies.json", "/typologies")):
        with open(os.path.join(HERE, name)) as f:
            for item in json.load(f):
                call("POST", path, item, admin=True)
                print(f"loaded {kind} {item['id']}")


def screen_transfer(tx):
    """Return the Osprey decision for one transfer."""
    return call("POST", "/evaluate", tx)


def case_file(evaluation_id):
    """Explain an alert from the stored evaluation: the typologies and rule contributions."""
    evaluation = call("GET", f"/evaluations/{evaluation_id}")
    lines = []
    for t in sorted(evaluation["typologyResults"], key=lambda t: t["typologyId"]):
        if not t["triggered"]:
            continue
        lines.append(f"  typology {t['typologyId']}: score {t['score']:.2f} >= threshold {t['threshold']:.2f}")
        for c in t["contributions"]:
            lines.append(
                f"    {c['ruleId']:<20} rule score {c['ruleScore']:.0f} x weight {c['weight']:.2f}"
                f" = {c['contribution']:.2f}"
            )
    return lines


def main():
    load_config()
    with open(os.path.join(HERE, "scenario.json")) as f:
        scenario = json.load(f)

    failures = 0
    for step in scenario:
        tx = step["tx"]
        res = screen_transfer(tx)
        ok = res["status"] == step["expect"]
        failures += not ok
        print()
        print(
            f"{tx['id']}  {tx['debtor']['id']} -> {tx['creditor']['id']}  "
            f"{tx['amount']['value']} {tx['amount']['currency']}"
        )
        print(f"  {res['status']}  score {res['score']:.2f}" + ("" if ok else f"   <-- expected {step['expect']}"))
        for reason in sorted(res.get("reasons", [])):
            print(f"  - {reason}")
        if res["status"] == "ALRT":
            print("  case file:")
            for line in case_file(res["evaluationId"]):
                print("  " + line)

    print()
    if failures:
        print(f"FAIL: {failures} transfer(s) did not get the expected decision")
        sys.exit(1)
    print(f"OK: all {len(scenario)} transfers got the expected decision")


if __name__ == "__main__":
    main()
