#!/usr/bin/env bash
# Stop before a sandbox setup can replace resources or use an occupied port.

set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-osprey-sandbox:local}"
CONTAINER_NAME="${CONTAINER_NAME:-osprey-sandbox}"
VOLUME_NAME="${VOLUME_NAME:-osprey-sandbox-data}"
HOST_PORT="${DOCKER_PORT:-${OSPREY_HOST_PORT:-}}"
PORT_VARIABLE="DOCKER_PORT"
conflict=false

if [[ -z "${DOCKER_PORT:-}" ]]; then
  PORT_VARIABLE="OSPREY_HOST_PORT"
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: Docker is required" >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "ERROR: Docker is not running" >&2
  exit 1
fi

if [[ -n "$HOST_PORT" ]] && (exec 3<>"/dev/tcp/127.0.0.1/$HOST_PORT") 2>/dev/null; then
  echo "ERROR: TCP port $HOST_PORT is already in use; choose an unused $PORT_VARIABLE" >&2
  conflict=true
fi

if docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
  echo "ERROR: Docker image already exists: $IMAGE_NAME" >&2
  conflict=true
fi

if docker container inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
  echo "ERROR: Docker container already exists: $CONTAINER_NAME" >&2
  conflict=true
fi

if docker volume inspect "$VOLUME_NAME" >/dev/null 2>&1; then
  echo "ERROR: Docker volume already exists: $VOLUME_NAME" >&2
  conflict=true
fi

if [[ "$conflict" == "true" ]]; then
  echo "Choose new names or manage the existing resources yourself. Nothing was changed." >&2
  exit 1
fi

if [[ -n "$HOST_PORT" ]]; then
  echo "Docker resource names and host port are available."
else
  echo "Docker image, container, and volume names are available."
fi
