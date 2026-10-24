#!/usr/bin/env bash
# Fetch the public OFAC SDN list for sanctions screening.
# Usage: scripts/fetch-ofac.sh [dir]   (default: data/ofac)
# Then run Osprey with OSPREY_SANCTIONS_DIR=<dir>. Restart to pick up a refresh.
set -euo pipefail
dir="${1:-data/ofac}"
base="https://sanctionslistservice.ofac.treas.gov/api/PublicationPreview/exports"
mkdir -p "$dir"
trap 'rm -f "$dir"/*.CSV.tmp' EXIT
# Download both before replacing either, so SDN.CSV and ALT.CSV stay a matched pair.
for f in SDN ALT; do
  curl -fsSL --retry 3 -o "$dir/$f.CSV.tmp" "$base/$f.CSV"
  [ -s "$dir/$f.CSV.tmp" ] || { echo "empty $f.CSV" >&2; exit 1; }
done
mv "$dir/SDN.CSV.tmp" "$dir/SDN.CSV"
mv "$dir/ALT.CSV.tmp" "$dir/ALT.CSV"
echo "OFAC list in $dir: $(wc -l <"$dir/SDN.CSV") SDN rows, $(wc -l <"$dir/ALT.CSV") aliases"
