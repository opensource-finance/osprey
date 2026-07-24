#!/usr/bin/env bash
# Prove the integration runner refuses to attach to an existing Osprey process.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_ROOT="$(mktemp -d -t osprey-port-guard.XXXXXX)"
BIN_PATH="$TEST_ROOT/osprey"
DB_PATH="$TEST_ROOT/existing.db"
LOG_PATH="$TEST_ROOT/existing.log"
TEST_PORT="${OSPREY_COLLISION_TEST_PORT:-$(ruby -rsocket -e 'server = TCPServer.new("127.0.0.1", 0); puts server.addr[1]; server.close')}"
EXISTING_PID=""

cleanup() {
  if [[ -n "$EXISTING_PID" ]] && kill -0 "$EXISTING_PID" 2>/dev/null; then
    kill "$EXISTING_PID" 2>/dev/null || true
    wait "$EXISTING_PID" 2>/dev/null || true
  fi
  rm -rf "$TEST_ROOT"
}
trap cleanup EXIT

go -C "$REPO_ROOT" build -o "$BIN_PATH" ./cmd/osprey

OSPREY_PORT="$TEST_PORT" \
OSPREY_ADMIN_TOKEN=existing-process-token \
OSPREY_SQLITE_PATH="$DB_PATH" \
  "$BIN_PATH" >"$LOG_PATH" 2>&1 &
EXISTING_PID=$!

for _ in $(seq 1 20); do
  if curl -fsS "http://localhost:$TEST_PORT/health" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! curl -fsS "http://localhost:$TEST_PORT/health" >/dev/null 2>&1; then
  echo "ERROR: collision fixture failed to start" >&2
  tail -n 50 "$LOG_PATH" >&2 || true
  exit 1
fi

set +e
output="$(
  OSPREY_TEST_PORT="$TEST_PORT" \
  OSPREY_ADMIN_TOKEN=integration-runner-token \
    "$REPO_ROOT/scripts/test-integration.sh" 2>&1
)"
exit_code=$?
set -e

if [[ "$exit_code" -eq 0 ]]; then
  echo "ERROR: integration runner accepted an occupied port" >&2
  exit 1
fi

expected="ERROR: http://localhost:$TEST_PORT/health is already responding; choose an unused OSPREY_TEST_PORT"
if ! grep -Fq "$expected" <<<"$output"; then
  echo "ERROR: integration runner did not report the occupied port clearly" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi

if grep -Fq "Seeding minimal integration rules" <<<"$output"; then
  echo "ERROR: integration runner seeded an existing service" >&2
  exit 1
fi

echo "Integration port guard passed."
