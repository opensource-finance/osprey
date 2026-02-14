#!/usr/bin/env bash
# Run Osprey integration tests against a clean local server.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

BIN_PATH="/tmp/osprey-integration-$$"
LOG_PATH="/tmp/osprey-integration-$$.log"
PID_FILE="/tmp/osprey-integration-$$.pid"

cleanup() {
  if [[ -f "$PID_FILE" ]]; then
    local pid
    pid="$(cat "$PID_FILE" 2>/dev/null || true)"
    if [[ -n "${pid:-}" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  fi

  rm -f \
    "$BIN_PATH" \
    "$LOG_PATH" \
    "$PID_FILE" \
    /tmp/osprey.db \
    /tmp/osprey.db-shm \
    /tmp/osprey.db-wal \
    /tmp/osprey.db-journal
}
trap cleanup EXIT

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq is required for seeding rules" >&2
  exit 1
fi

echo "Building Osprey binary..."
go -C "$REPO_ROOT" build -o "$BIN_PATH" ./cmd/osprey

echo "Starting Osprey from /tmp (clean sqlite state)..."
(
  cd /tmp
  "$BIN_PATH" >"$LOG_PATH" 2>&1 &
  echo $! >"$PID_FILE"
)

echo "Waiting for health endpoint..."
for _ in $(seq 1 30); do
  if curl -sf http://localhost:8080/health >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! curl -sf http://localhost:8080/health >/dev/null 2>&1; then
  echo "ERROR: Osprey failed to start" >&2
  tail -n 50 "$LOG_PATH" >&2 || true
  exit 1
fi

echo "Seeding minimal integration rules..."
OSPREY_URL="http://localhost:8080" "$REPO_ROOT/scripts/seed-rules.sh"

echo "Running integration tests..."
go -C "$REPO_ROOT" test -tags=integration -v ./tests/integration/...

echo "Integration tests passed."
