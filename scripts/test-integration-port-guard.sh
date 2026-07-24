#!/usr/bin/env bash
# Prove the integration runner refuses to use any occupied TCP port.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_ROOT="$(mktemp -d -t osprey-integration-port-guard.XXXXXX)"
PORT_FILE="$TEST_ROOT/port"
REQUESTED_PORT="${OSPREY_COLLISION_TEST_PORT:-0}"
EXISTING_PID=""

cleanup() {
  if [[ -n "$EXISTING_PID" ]] && kill -0 "$EXISTING_PID" 2>/dev/null; then
    kill "$EXISTING_PID" 2>/dev/null || true
    wait "$EXISTING_PID" 2>/dev/null || true
  fi
  rm -rf "$TEST_ROOT"
}
trap cleanup EXIT

REQUESTED_PORT="$REQUESTED_PORT" PORT_FILE="$PORT_FILE" ruby -rsocket -e '
server = TCPServer.new("127.0.0.1", Integer(ENV.fetch("REQUESTED_PORT")))
File.write(ENV.fetch("PORT_FILE"), server.addr[1])
loop do
  client = nil
  begin
    client = server.accept
    request_line = client.gets
    if request_line
      client.write("HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
    end
  rescue IOError, SystemCallError
    # A plain TCP probe may disconnect without sending an HTTP request.
  ensure
    client&.close
  end
end
' >/dev/null 2>&1 &
EXISTING_PID=$!

for _ in $(seq 1 20); do
  if [[ -s "$PORT_FILE" ]]; then
    break
  fi
  sleep 0.1
done

if [[ ! -s "$PORT_FILE" ]] || ! kill -0 "$EXISTING_PID" 2>/dev/null; then
  echo "ERROR: collision fixture failed to start" >&2
  exit 1
fi
TEST_PORT="$(cat "$PORT_FILE")"

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

expected="ERROR: TCP port $TEST_PORT is already in use; choose an unused OSPREY_TEST_PORT"
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
