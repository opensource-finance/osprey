#!/usr/bin/env bash
# Prove sandbox setup refuses to reuse an existing Docker resource.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUFFIX="$(ruby -rsecurerandom -e 'print SecureRandom.hex(12)')"
IMAGE_NAME="osprey:resource-guard-test-$SUFFIX"
CONTAINER_NAME="osprey-resource-guard-test-$SUFFIX"
VOLUME_NAME="osprey-resource-guard-test-$SUFFIX"
volume_created=false

cleanup() {
  local label

  if [[ "$volume_created" == "true" ]]; then
    label="$(docker volume inspect --format '{{index .Labels "osprey-test-id"}}' "$VOLUME_NAME" 2>/dev/null || true)"
    if [[ "$label" == "$SUFFIX" ]]; then
      docker volume rm "$VOLUME_NAME" >/dev/null 2>&1 || true
    fi
  fi
}
trap cleanup EXIT

if docker volume inspect "$VOLUME_NAME" >/dev/null 2>&1; then
  echo "ERROR: generated Docker test volume already exists: $VOLUME_NAME" >&2
  exit 1
fi

docker volume create --label "osprey-test-id=$SUFFIX" "$VOLUME_NAME" >/dev/null
label="$(docker volume inspect --format '{{index .Labels "osprey-test-id"}}' "$VOLUME_NAME" 2>/dev/null || true)"
if [[ "$label" != "$SUFFIX" ]]; then
  echo "ERROR: Docker test volume was not created safely: $VOLUME_NAME" >&2
  exit 1
fi
volume_created=true

set +e
output="$(
  IMAGE_NAME="$IMAGE_NAME" \
  CONTAINER_NAME="$CONTAINER_NAME" \
  VOLUME_NAME="$VOLUME_NAME" \
    "$SCRIPT_DIR/check-docker-resource-names.sh" 2>&1
)"
exit_code=$?
set -e

if [[ "$exit_code" -eq 0 ]]; then
  echo "ERROR: Docker resource guard accepted an existing volume" >&2
  exit 1
fi

expected="ERROR: Docker volume already exists: $VOLUME_NAME"
if ! grep -Fq "$expected" <<<"$output"; then
  echo "ERROR: Docker resource guard did not report the existing volume clearly" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi

label="$(docker volume inspect --format '{{index .Labels "osprey-test-id"}}' "$VOLUME_NAME")"
if [[ "$label" != "$SUFFIX" ]]; then
  echo "ERROR: Docker resource guard replaced or changed the existing volume" >&2
  exit 1
fi

if ! grep -Fq './scripts/check-docker-resource-names.sh' "$SCRIPT_DIR/assure-sandbox.sh"; then
  echo "ERROR: sandbox assurance does not run the Docker resource guard" >&2
  exit 1
fi

if ! grep -Fq './scripts/check-docker-resource-names.sh' "$SCRIPT_DIR/../docs/SANDBOX.md"; then
  echo "ERROR: sandbox guide does not run the Docker resource guard" >&2
  exit 1
fi

if ! grep -Fq -- '-p "127.0.0.1:${DOCKER_PORT}:8080"' "$SCRIPT_DIR/assure-sandbox.sh"; then
  echo "ERROR: sandbox assurance does not bind its Docker port to loopback" >&2
  exit 1
fi

echo "Docker resource guard passed."
