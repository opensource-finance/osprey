#!/usr/bin/env bash
# Print Coolify build arguments for the public Osprey sandbox.

set -euo pipefail

version="${VERSION:-sandbox-$(date -u +%Y%m%d)}"
commit="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
build_date="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

cat <<EOF
# Coolify build arguments
VERSION=$version
COMMIT=$commit
BUILD_DATE=$build_date

# Use this during post-deploy verification
EXPECTED_VERSION=$version
EOF
