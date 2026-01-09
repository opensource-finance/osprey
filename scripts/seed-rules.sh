#!/bin/bash
# Seed MINIMAL rules for integration testing
# Usage: ./scripts/seed-rules.sh
#
# Creates 3 simple rules for integration tests:
#   - high-value-001: Flags transfers > $10,000
#   - same-account-001: Flags self-transfers (CRITICAL)
#   - amount-check-001: Basic amount validation
#
# For production rules, use: ./scripts/seed-starter-kit.sh
# For PaySim benchmark, use: ./scripts/seed-paysim.sh

set -e

BASE_URL="${OSPREY_URL:-http://localhost:8080}"
TENANT_ID="${OSPREY_TENANT:-default}"

echo "Seeding rules to $BASE_URL (tenant: $TENANT_ID)..."
echo ""

# Check if server is running
if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
  echo "ERROR: Server not reachable at $BASE_URL"
  echo "Start the server first: go run ./cmd/osprey"
  exit 1
fi

# Rule 1: High value transfer check (weight 0.3 - contributes to score)
echo "Creating high-value-001..."
curl -s -X POST "$BASE_URL/rules" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d '{
    "id": "high-value-001",
    "name": "High Value Transfer Check",
    "description": "Flags transfers above 10000",
    "expression": "amount > 10000.0 ? 1.0 : 0.0",
    "bands": [
      {"lowerLimit": 0.0, "upperLimit": 1.0, "subRuleRef": ".pass", "reason": "Normal amount"},
      {"lowerLimit": 1.0, "upperLimit": null, "subRuleRef": ".review", "reason": "High value transfer"}
    ],
    "weight": 0.3,
    "enabled": true
  }' | jq -r '.message // .error // .'

# Rule 2: Same account check (weight 1.0 - CRITICAL, triggers .fail)
# This rule alone must trigger ALRT when debtor_id == creditor_id
echo "Creating same-account-001..."
curl -s -X POST "$BASE_URL/rules" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d '{
    "id": "same-account-001",
    "name": "Same Account Transfer Check",
    "description": "Flags transfers where debtor and creditor are the same - critical alert",
    "expression": "debtor_id == creditor_id ? 1.0 : 0.0",
    "bands": [
      {"lowerLimit": 0.0, "upperLimit": 1.0, "subRuleRef": ".pass", "reason": "Different parties"},
      {"lowerLimit": 1.0, "upperLimit": null, "subRuleRef": ".fail", "reason": "Same account transfer"}
    ],
    "weight": 1.0,
    "enabled": true
  }' | jq -r '.message // .error // .'

# Rule 3: Amount check (weight 0.3 - basic validation)
echo "Creating amount-check-001..."
curl -s -X POST "$BASE_URL/rules" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: $TENANT_ID" \
  -d '{
    "id": "amount-check-001",
    "name": "Amount Validation",
    "description": "Basic amount validation - passes for positive amounts",
    "expression": "amount > 0.0 ? 0.0 : 1.0",
    "bands": [
      {"lowerLimit": 0.0, "upperLimit": 0.5, "subRuleRef": ".pass", "reason": "Valid amount"},
      {"lowerLimit": 0.5, "upperLimit": null, "subRuleRef": ".fail", "reason": "Invalid amount"}
    ],
    "weight": 0.3,
    "enabled": true
  }' | jq -r '.message // .error // .'

echo ""
echo "Reloading rules into engine..."
curl -s -X POST "$BASE_URL/rules/reload" \
  -H "X-Tenant-ID: $TENANT_ID" | jq -r '.message // .error // .'

echo ""
echo "Verifying loaded rules:"
curl -s "$BASE_URL/rules" \
  -H "X-Tenant-ID: $TENANT_ID" | jq '{count: .count, rules: [.rules[] | {id, weight, enabled}]}'

echo ""
echo "Done! You can now run:"
echo "  - k6 run k6/quick-test.js"
echo "  - go test ./tests/integration/..."
