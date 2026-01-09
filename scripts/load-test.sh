#!/bin/bash
# Production Load Test Runner
# Usage: ./scripts/load-test.sh [local|docker|remote URL]
#
# Examples:
#   ./scripts/load-test.sh local        # Test against localhost:8080
#   ./scripts/load-test.sh docker       # Start Docker stack and test
#   ./scripts/load-test.sh https://api.example.com  # Test remote server

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
DEFAULT_MAX_VUS=100
DOCKER_COMPOSE_FILE="docker-compose.yml"

print_header() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║              OSPREY PRODUCTION LOAD TEST                     ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

check_dependencies() {
    echo -e "${YELLOW}Checking dependencies...${NC}"

    if ! command -v k6 &> /dev/null; then
        echo -e "${RED}k6 is not installed.${NC}"
        echo "Install with: brew install k6 (macOS) or https://k6.io/docs/getting-started/installation/"
        exit 1
    fi
    echo -e "  ${GREEN}✓${NC} k6 installed"

    if ! command -v docker &> /dev/null; then
        echo -e "${YELLOW}  ⚠ Docker not found (needed for docker mode)${NC}"
    else
        echo -e "  ${GREEN}✓${NC} Docker installed"
    fi

    if ! command -v jq &> /dev/null; then
        echo -e "${YELLOW}  ⚠ jq not found (optional, for JSON parsing)${NC}"
    else
        echo -e "  ${GREEN}✓${NC} jq installed"
    fi

    echo ""
}

wait_for_healthy() {
    local url=$1
    local max_attempts=30
    local attempt=0

    echo -e "${YELLOW}Waiting for server to be healthy...${NC}"

    while [ $attempt -lt $max_attempts ]; do
        if curl -s "${url}/health" > /dev/null 2>&1; then
            local status=$(curl -s "${url}/health" | jq -r '.status // "unknown"' 2>/dev/null || echo "ok")
            if [ "$status" = "healthy" ] || [ "$status" = "ok" ]; then
                echo -e "  ${GREEN}✓${NC} Server is healthy"
                return 0
            fi
        fi
        attempt=$((attempt + 1))
        echo -e "  Waiting... (attempt $attempt/$max_attempts)"
        sleep 2
    done

    echo -e "${RED}Server failed to become healthy${NC}"
    return 1
}

seed_rules() {
    local url=$1

    echo -e "${YELLOW}Seeding rules...${NC}"

    if [ -f "scripts/seed-rules.sh" ]; then
        OSPREY_URL="$url" ./scripts/seed-rules.sh > /dev/null 2>&1 || true
        echo -e "  ${GREEN}✓${NC} Rules seeded"
    else
        echo -e "  ${YELLOW}⚠${NC} Seed script not found, skipping"
    fi
}

run_local() {
    local url="http://localhost:8080"

    echo -e "${BLUE}Mode: LOCAL${NC}"
    echo -e "Target: $url"
    echo ""

    if ! curl -s "${url}/health" > /dev/null 2>&1; then
        echo -e "${RED}Server not running at $url${NC}"
        echo "Start it with: go run ./cmd/osprey"
        exit 1
    fi

    seed_rules "$url"
    run_load_test "$url"
}

run_docker() {
    local url="http://localhost:8080"

    echo -e "${BLUE}Mode: DOCKER (Production Stack)${NC}"
    echo ""

    if [ ! -f "$DOCKER_COMPOSE_FILE" ]; then
        echo -e "${RED}docker-compose.yml not found${NC}"
        exit 1
    fi

    echo -e "${YELLOW}Starting production stack...${NC}"
    docker-compose up -d --build

    wait_for_healthy "$url"
    seed_rules "$url"

    # Show stack status
    echo ""
    echo -e "${YELLOW}Stack Status:${NC}"
    docker-compose ps
    echo ""

    run_load_test "$url"

    echo ""
    echo -e "${YELLOW}Stopping stack...${NC}"
    docker-compose down
}

run_remote() {
    local url=$1

    echo -e "${BLUE}Mode: REMOTE${NC}"
    echo -e "Target: $url"
    echo ""

    wait_for_healthy "$url"
    run_load_test "$url"
}

run_load_test() {
    local url=$1
    local max_vus=${MAX_VUS:-$DEFAULT_MAX_VUS}
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local output_dir="k6/results"

    # Create output directory
    mkdir -p "$output_dir"

    echo ""
    echo -e "${YELLOW}Running load test...${NC}"
    echo -e "  Target:  $url"
    echo -e "  Max VUs: $max_vus"
    echo -e "  Output:  $output_dir/"
    echo ""

    # Run with multiple outputs
    k6 run \
        -e BASE_URL="$url" \
        -e MAX_VUS="$max_vus" \
        --out json="$output_dir/results_${timestamp}.json" \
        --out csv="$output_dir/results_${timestamp}.csv" \
        k6/production-load-test.js \
        2>&1 | tee "$output_dir/console_${timestamp}.log"

    echo ""
    echo -e "${GREEN}Load test complete!${NC}"
    echo ""
    echo -e "${YELLOW}Results saved to:${NC}"
    echo -e "  JSON:    $output_dir/results_${timestamp}.json"
    echo -e "  CSV:     $output_dir/results_${timestamp}.csv"
    echo -e "  Console: $output_dir/console_${timestamp}.log"
    echo ""

    # Generate summary
    if [ -f "$output_dir/results_${timestamp}.json" ]; then
        echo -e "${YELLOW}Quick Summary:${NC}"
        # Extract key metrics from JSON (last line contains summary)
        tail -1 "$output_dir/results_${timestamp}.json" 2>/dev/null | jq -r '
            select(.type == "Point" and .metric == "http_reqs") |
            "  Total Requests: \(.data.value)"
        ' 2>/dev/null || true
    fi
}

# Main
print_header
check_dependencies

case "${1:-local}" in
    local)
        run_local
        ;;
    docker)
        run_docker
        ;;
    http*|https*)
        run_remote "$1"
        ;;
    *)
        echo "Usage: $0 [local|docker|URL]"
        echo ""
        echo "Examples:"
        echo "  $0 local                    # Test localhost:8080"
        echo "  $0 docker                   # Start Docker stack and test"
        echo "  $0 https://api.example.com  # Test remote server"
        echo ""
        echo "Environment variables:"
        echo "  MAX_VUS=100    # Maximum virtual users"
        exit 1
        ;;
esac
