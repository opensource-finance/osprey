#!/usr/bin/env bash
# Prove the integration runner refuses to use any occupied TCP port.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_PORT="${OSPREY_COLLISION_TEST_PORT:-$(ruby -rsocket -e 'server = TCPServer.new("127.0.0.1", 0); puts server.addr[1]; server.close')}"
EXISTING_PID=""

cleanup() {
  if [[ -n "$EXISTING_PID" ]] && kill -0 "$EXISTING_PID" 2>/dev/null; then
    kill "$EXISTING_PID" 2>/dev/null || true
    wait "$EXISTING_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

TEST_PORT="$TEST_PORT" ruby -rsocket -e '
server = TCPServer.new("127.0.0.1", ENV.fetch("TEST_PORT"))
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
  if ruby -rsocket -e 'socket = TCPSocket.new("127.0.0.1", ARGV.fetch(0)); socket.close' "$TEST_PORT" 2>/dev/null; then
    break
  fi
  sleep 0.1
done

if ! ruby -rsocket -e 'socket = TCPSocket.new("127.0.0.1", ARGV.fetch(0)); socket.close' "$TEST_PORT" 2>/dev/null; then
  echo "ERROR: collision fixture failed to start" >&2
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
