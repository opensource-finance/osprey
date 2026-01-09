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

# Counters
RULES_CREATED=0
RULES_SKIPPED=0
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
    response=$(curl -s -X POST "$BASE_URL/rules" \
        -H "Content-Type: application/json" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -d "$rule")

    if echo "$response" | jq -e '.error' > /dev/null 2>&1; then
        error=$(echo "$response" | jq -r '.error')
        if echo "$error" | grep -qi "already exists"; then
            echo -e "  ${YELLOW}○${NC} $rule_id (exists)"
            ((RULES_SKIPPED++)) || true
        else
            echo -e "  ${RED}✗${NC} $rule_id: $error"
            ((RULES_FAILED++)) || true
        fi
    else
        echo -e "  ${GREEN}✓${NC} $rule_id"
        ((RULES_CREATED++)) || true
    fi
done < <(jq -c '.rules[]' "$RULES_FILE")

# Reload
echo ""
echo -e "${YELLOW}Reloading rules...${NC}"
reload_response=$(curl -s -X POST "$BASE_URL/rules/reload" -H "X-Tenant-ID: $TENANT_ID")
loaded_count=$(echo "$reload_response" | jq -r '.count // 0')
echo -e "${GREEN}✓${NC} $loaded_count rules loaded"

# Summary
echo ""
echo -e "${BLUE}Summary:${NC}"
echo -e "  Created: ${GREEN}$RULES_CREATED${NC}"
echo -e "  Skipped: ${YELLOW}$RULES_SKIPPED${NC}"
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
