#!/usr/bin/env bash
# Prove Docker sandbox setup stops when its host port is already occupied.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_ROOT="$(mktemp -d -t osprey-docker-port-guard.XXXXXX)"
PORT_FILE="$TEST_ROOT/port"
SUFFIX="$(ruby -rsecurerandom -e 'print SecureRandom.hex(12)')"
LISTENER_PID=""

cleanup() {
  if [[ -n "$LISTENER_PID" ]] && kill -0 "$LISTENER_PID" 2>/dev/null; then
    kill "$LISTENER_PID" 2>/dev/null || true
    wait "$LISTENER_PID" 2>/dev/null || true
  fi
  rm -rf "$TEST_ROOT"
}
trap cleanup EXIT

PORT_FILE="$PORT_FILE" ruby -rsocket -e '
server = TCPServer.new("127.0.0.1", 0)
File.write(ENV.fetch("PORT_FILE"), server.addr[1])
loop do
  client = server.accept
  client.close
end
' >/dev/null 2>&1 &
LISTENER_PID=$!

for _ in $(seq 1 20); do
  if [[ -s "$PORT_FILE" ]]; then
    break
  fi
  sleep 0.1
done

if [[ ! -s "$PORT_FILE" ]] || ! kill -0 "$LISTENER_PID" 2>/dev/null; then
  echo "ERROR: Docker port collision fixture failed to start" >&2
  exit 1
fi
TEST_PORT="$(cat "$PORT_FILE")"

set +e
output="$(
  IMAGE_NAME="osprey:docker-port-guard-test-$SUFFIX" \
  CONTAINER_NAME="osprey-docker-port-guard-test-$SUFFIX" \
  VOLUME_NAME="osprey-docker-port-guard-test-$SUFFIX" \
  DOCKER_PORT="$TEST_PORT" \
    "$SCRIPT_DIR/check-docker-resource-names.sh" 2>&1
)"
exit_code=$?
set -e

if [[ "$exit_code" -eq 0 ]]; then
  echo "ERROR: Docker resource guard accepted an occupied port" >&2
  exit 1
fi

expected="ERROR: TCP port $TEST_PORT is already in use; choose an unused DOCKER_PORT"
if ! grep -Fq "$expected" <<<"$output"; then
  echo "ERROR: Docker resource guard did not report the occupied port clearly" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi

echo "Docker port guard passed."
