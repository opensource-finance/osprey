#!/usr/bin/env bash
# Start a clean Osprey server, load the rules, and replay the transfer scenario.
# Needs Go 1.26+ and Python 3. Run it from any directory.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
port="${PORT:-18082}"
data="$(mktemp -d)"

export OSPREY_ADMIN_TOKEN="${OSPREY_ADMIN_TOKEN:-example-admin-token}"
export OSPREY_URL="http://localhost:$port"

(cd "$repo" && go build -o "$data/osprey" ./cmd/osprey)
OSPREY_PORT="$port" OSPREY_MODE=compliance OSPREY_SQLITE_PATH="$data/osprey.db" \
  "$data/osprey" >"$data/server.log" 2>&1 &
server=$!
trap 'kill "$server" 2>/dev/null || true; rm -rf "$data"' EXIT

for _ in $(seq 1 50); do
  curl -fsS "$OSPREY_URL/health" >/dev/null 2>&1 && break
  sleep 0.2
done
curl -fsS "$OSPREY_URL/health" >/dev/null || { cat "$data/server.log"; exit 1; }

python3 "$here/remittance_monitor.py"
