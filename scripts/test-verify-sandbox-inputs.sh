#!/usr/bin/env bash
# Verify the public sandbox checker refuses an implicit deployment target.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

set +e
output="$(
  env -u OSPREY_URL \
    OSPREY_ADMIN_TOKEN=input-test-token \
    "$REPO_ROOT/scripts/verify-sandbox.sh" 2>&1
)"
exit_code=$?
set -e

if [[ "$exit_code" -eq 0 ]]; then
  echo "ERROR: sandbox verifier accepted a missing OSPREY_URL" >&2
  exit 1
fi

if ! grep -Fq "ERROR: OSPREY_URL is required" <<<"$output"; then
  echo "ERROR: sandbox verifier did not explain the missing OSPREY_URL" >&2
  printf '%s\n' "$output" >&2
  exit 1
fi

echo "Sandbox verifier input guard passed."
