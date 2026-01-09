#!/bin/bash
# Osprey Starter Kit - FATF-aligned Rules and Typologies
#
# This script loads production-ready rules and typologies based on
# public FATF (Financial Action Task Force) guidance.
#
# Usage:
#   ./scripts/seed-starter-kit.sh              # Default: Detection mode rules
#   ./scripts/seed-starter-kit.sh --compliance # Include typologies for Compliance mode
#   ./scripts/seed-starter-kit.sh --tenant mycompany  # Custom tenant ID
#
# Sources:
#   - FATF Methods and Trends: https://www.fatf-gafi.org/en/topics/methods-and-trends.html
#   - FATF Recommendations: https://www.fatf-gafi.org/en/publications/Fatfrecommendations.html

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
BASE_URL="${OSPREY_URL:-http://localhost:8080}"
TENANT_ID="${OSPREY_TENANT:-default}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIGS_DIR="$SCRIPT_DIR/../configs"

# Counters for tracking success/failure
RULES_CREATED=0
RULES_SKIPPED=0
RULES_FAILED=0
TYPOLOGIES_CREATED=0
TYPOLOGIES_SKIPPED=0
TYPOLOGIES_FAILED=0

# Flags
INCLUDE_TYPOLOGIES=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --compliance)
            INCLUDE_TYPOLOGIES=true
            shift
            ;;
        --tenant)
            TENANT_ID="$2"
            shift 2
            ;;
        --help|-h)
            echo "Osprey Starter Kit - FATF-aligned Rules and Typologies"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --compliance      Include typologies (for Compliance mode)"
            echo "  --tenant <id>     Set tenant ID (default: default)"
            echo "  --help            Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  OSPREY_URL        Base URL (default: http://localhost:8080)"
            echo "  OSPREY_TENANT     Tenant ID (default: default)"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

print_header() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║           OSPREY STARTER KIT - FATF ALIGNED                  ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "  Target:     ${CYAN}$BASE_URL${NC}"
    echo -e "  Tenant:     ${CYAN}$TENANT_ID${NC}"
    echo -e "  Typologies: ${CYAN}$INCLUDE_TYPOLOGIES${NC}"
    echo ""
}

check_server() {
    echo -e "${YELLOW}Checking server...${NC}"

    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        echo -e "${RED}ERROR: Server not reachable at $BASE_URL${NC}"
        echo "Start Osprey first: go run ./cmd/osprey"
        exit 1
    fi

    local health_response=$(curl -s "$BASE_URL/health")
    local mode=$(echo "$health_response" | jq -r '.mode // "unknown"')
    local status=$(echo "$health_response" | jq -r '.status // "unknown"')

    echo -e "  ${GREEN}✓${NC} Server healthy (mode: $mode, status: $status)"

    if [ "$INCLUDE_TYPOLOGIES" = true ] && [ "$mode" != "compliance" ]; then
        echo -e "  ${YELLOW}⚠${NC}  Server in Detection mode - typologies won't be evaluated"
        echo -e "      Set OSPREY_MODE=compliance to enable typology evaluation"
    fi
    echo ""
}

check_dependencies() {
    if ! command -v jq &> /dev/null; then
        echo -e "${RED}ERROR: jq is required but not installed${NC}"
        echo "Install with: brew install jq (macOS) or apt install jq (Linux)"
        exit 1
    fi
}

create_rule() {
    local rule_json="$1"
    local rule_id=$(echo "$rule_json" | jq -r '.id')

    local response=$(curl -s -X POST "$BASE_URL/rules" \
        -H "Content-Type: application/json" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -d "$rule_json" 2>&1)

    if echo "$response" | jq -e '.error' > /dev/null 2>&1; then
        local error=$(echo "$response" | jq -r '.error // .message // .')
        if echo "$error" | grep -qi "already exists"; then
            echo -e "  ${YELLOW}○${NC} $rule_id (already exists)"
            ((RULES_SKIPPED++)) || true
        else
            echo -e "  ${RED}✗${NC} $rule_id: $error"
            ((RULES_FAILED++)) || true
        fi
    elif echo "$response" | jq -e '.rule' > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} $rule_id"
        ((RULES_CREATED++)) || true
    else
        echo -e "  ${RED}✗${NC} $rule_id: unexpected response"
        ((RULES_FAILED++)) || true
    fi
}

create_typology() {
    local typology_json="$1"
    local typology_id=$(echo "$typology_json" | jq -r '.id')

    local response=$(curl -s -X POST "$BASE_URL/typologies" \
        -H "Content-Type: application/json" \
        -H "X-Tenant-ID: $TENANT_ID" \
        -d "$typology_json" 2>&1)

    if echo "$response" | jq -e '.error' > /dev/null 2>&1; then
        local error=$(echo "$response" | jq -r '.error // .message // .')
        if echo "$error" | grep -qi "already exists"; then
            echo -e "  ${YELLOW}○${NC} $typology_id (already exists)"
            ((TYPOLOGIES_SKIPPED++)) || true
        elif echo "$error" | grep -qi "does not exist in rule engine"; then
            echo -e "  ${RED}✗${NC} $typology_id: Missing rule dependency"
            echo -e "      $error"
            ((TYPOLOGIES_FAILED++)) || true
        else
            echo -e "  ${RED}✗${NC} $typology_id: $error"
            ((TYPOLOGIES_FAILED++)) || true
        fi
    elif echo "$response" | jq -e '.typology' > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} $typology_id"
        ((TYPOLOGIES_CREATED++)) || true
    else
        echo -e "  ${RED}✗${NC} $typology_id: unexpected response"
        ((TYPOLOGIES_FAILED++)) || true
    fi
}

load_rules() {
    local rules_file="$CONFIGS_DIR/rules/fatf-rules.json"

    if [ ! -f "$rules_file" ]; then
        echo -e "${RED}ERROR: Rules file not found: $rules_file${NC}"
        exit 1
    fi

    echo -e "${YELLOW}Loading FATF-aligned rules...${NC}"

    local count=$(jq '.rules | length' "$rules_file")
    echo -e "  Found $count rules in config"
    echo ""

    # Read each rule and create it
    while IFS= read -r rule; do
        create_rule "$rule"
    done < <(jq -c '.rules[]' "$rules_file")

    echo ""
    echo -e "${YELLOW}Reloading rule engine...${NC}"
    local reload_response=$(curl -s -X POST "$BASE_URL/rules/reload" -H "X-Tenant-ID: $TENANT_ID")
    local loaded_count=$(echo "$reload_response" | jq -r '.count // 0')
    echo -e "  ${GREEN}✓${NC} $loaded_count rules loaded into engine"

    # Verify rules actually loaded
    if [ "$loaded_count" -eq 0 ] && [ "$RULES_CREATED" -gt 0 ]; then
        echo -e "  ${RED}⚠${NC}  Warning: Rules created but none loaded - check server logs"
    fi
}

load_typologies() {
    local typologies_file="$CONFIGS_DIR/typologies/fatf-typologies.json"

    if [ ! -f "$typologies_file" ]; then
        echo -e "${RED}ERROR: Typologies file not found: $typologies_file${NC}"
        exit 1
    fi

    # Verify rules are loaded first
    local loaded_rules=$(curl -s "$BASE_URL/rules" -H "X-Tenant-ID: $TENANT_ID" | jq -r '.count // 0')
    if [ "$loaded_rules" -eq 0 ]; then
        echo -e "${RED}ERROR: No rules loaded. Load rules before typologies.${NC}"
        exit 1
    fi

    echo ""
    echo -e "${YELLOW}Loading FATF-aligned typologies...${NC}"

    local count=$(jq '.typologies | length' "$typologies_file")
    echo -e "  Found $count typologies in config"
    echo -e "  Verifying against $loaded_rules loaded rules..."
    echo ""

    # Read each typology and create it
    while IFS= read -r typology; do
        create_typology "$typology"
    done < <(jq -c '.typologies[]' "$typologies_file")

    echo ""
    echo -e "${YELLOW}Reloading typology engine...${NC}"
    local reload_response=$(curl -s -X POST "$BASE_URL/typologies/reload" -H "X-Tenant-ID: $TENANT_ID")
    local loaded_count=$(echo "$reload_response" | jq -r '.count // 0')
    echo -e "  ${GREEN}✓${NC} $loaded_count typologies loaded into engine"
}

print_summary() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║                         SUMMARY                              ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Rules summary
    echo -e "  ${CYAN}Rules:${NC}"
    echo -e "    Created:  ${GREEN}$RULES_CREATED${NC}"
    echo -e "    Skipped:  ${YELLOW}$RULES_SKIPPED${NC} (already exist)"
    echo -e "    Failed:   ${RED}$RULES_FAILED${NC}"

    if [ "$INCLUDE_TYPOLOGIES" = true ]; then
        echo ""
        echo -e "  ${CYAN}Typologies:${NC}"
        echo -e "    Created:  ${GREEN}$TYPOLOGIES_CREATED${NC}"
        echo -e "    Skipped:  ${YELLOW}$TYPOLOGIES_SKIPPED${NC} (already exist)"
        echo -e "    Failed:   ${RED}$TYPOLOGIES_FAILED${NC}"
    fi

    # Final status
    echo ""
    local total_failed=$((RULES_FAILED + TYPOLOGIES_FAILED))
    if [ "$total_failed" -gt 0 ]; then
        echo -e "  ${RED}⚠ PARTIAL FAILURE: $total_failed items failed to load${NC}"
        echo ""
        echo "  Review errors above and fix before using in production."
        exit 1
    else
        echo -e "  ${GREEN}✓ Starter kit loaded successfully!${NC}"
    fi

    echo ""
    echo "Next steps:"
    echo "  - Test: curl -X POST $BASE_URL/evaluate -H 'Content-Type: application/json' -H 'X-Tenant-ID: $TENANT_ID' -d '{...}'"
    echo "  - View rules: curl $BASE_URL/rules -H 'X-Tenant-ID: $TENANT_ID'"
    if [ "$INCLUDE_TYPOLOGIES" = true ]; then
        echo "  - View typologies: curl $BASE_URL/typologies -H 'X-Tenant-ID: $TENANT_ID'"
    fi
    echo ""
}

# Main
print_header
check_dependencies
check_server
load_rules

if [ "$INCLUDE_TYPOLOGIES" = true ]; then
    load_typologies
fi

print_summary
