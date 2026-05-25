#!/bin/bash
# Seed PaySim-optimized rules for benchmark testing
#
# Usage:
#   ./scripts/seed-paysim.sh
#   ./scripts/seed-paysim.sh --tenant mycompany
#
# These rules are specifically tuned for the PaySim fraud detection
# benchmark and achieve ~96% recall with 100% precision.

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration - use same default as seed-starter-kit.sh for consistency
BASE_URL="${OSPREY_URL:-http://localhost:8080}"
TENANT_ID="${OSPREY_TENANT:-default}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RULES_FILE="$SCRIPT_DIR/../configs/rules/paysim-rules.json"
ADMIN_HEADER=()
if [ -n "${OSPREY_ADMIN_TOKEN:-}" ]; then
    ADMIN_HEADER=(-H "Authorization: Bearer ${OSPREY_ADMIN_TOKEN}")
fi

# Counters
RULES_CREATED=0
RULES_FAILED=0

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --tenant)
            TENANT_ID="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [--tenant <id>]"
            exit 0
            ;;
        *)
            shift
            ;;
    esac
done

echo -e "${BLUE}╔═══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          OSPREY PAYSIM BENCHMARK RULES                        ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Target: $BASE_URL"
echo "Tenant: $TENANT_ID"
echo ""

# Check dependencies
if ! command -v jq &> /dev/null; then
    echo -e "${RED}ERROR: jq required (brew install jq)${NC}"
    exit 1
fi

# Check server
if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
    echo -e "${RED}ERROR: Server not running at $BASE_URL${NC}"
    exit 1
fi
echo -e "${GREEN}✓${NC} Server healthy"

# Check rules file
if [ ! -f "$RULES_FILE" ]; then
    echo -e "${RED}ERROR: Rules file not found: $RULES_FILE${NC}"
    exit 1
fi

# Load rules
echo ""
echo -e "${YELLOW}Loading PaySim rules...${NC}"

while IFS= read -r rule; do
    rule_id=$(echo "$rule" | jq -r '.id')
    response=$(curl -s -w '\n%{http_code}' -X POST "$BASE_URL/rules" \
        -H "Content-Type: application/json" \
        -H "X-Tenant-ID: $TENANT_ID" \
        "${ADMIN_HEADER[@]}" \
        -d "$rule")
    http_code="${response##*$'\n'}"
    body="${response%$'\n'*}"

    if [[ "$http_code" =~ ^2 ]] && echo "$body" | jq -e '.rule' > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} $rule_id"
        ((RULES_CREATED++)) || true
    else
        error=$(echo "$body" | jq -r '.error // .message // .' 2>/dev/null || printf '%s' "$body")
        echo -e "  ${RED}✗${NC} $rule_id: HTTP $http_code: $error"
        ((RULES_FAILED++)) || true
    fi
done < <(jq -c '.rules[]' "$RULES_FILE")

echo ""
echo -e "${YELLOW}Verifying active rules...${NC}"
rules_response=$(curl -s "$BASE_URL/rules" -H "X-Tenant-ID: $TENANT_ID")
loaded_count=$(echo "$rules_response" | jq -r '.count // 0')
echo -e "${GREEN}✓${NC} $loaded_count active rules"

# Summary
echo ""
echo -e "${BLUE}Summary:${NC}"
echo -e "  Loaded:  ${GREEN}$RULES_CREATED${NC}"
echo -e "  Failed:  ${RED}$RULES_FAILED${NC}"

if [ "$RULES_FAILED" -gt 0 ]; then
    echo ""
    echo -e "${RED}⚠ Some rules failed to load${NC}"
    exit 1
fi

echo ""
echo "Ready! Run benchmark with:"
echo "  ./benchmark -csv ../learning/PS_*.csv -tenant $TENANT_ID -limit 50000"
echo ""
