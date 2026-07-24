#!/usr/bin/env bash
# Verify an Osprey sandbox deployment through its public HTTP API.

set -euo pipefail

BASE_URL="${OSPREY_URL:-}"
TENANT_ID="${TENANT_ID:-sandbox-verification}"
ADMIN_TOKEN="${OSPREY_ADMIN_TOKEN:-}"
EXPECTED_STATUS="${EXPECTED_STATUS:-}"
EXPECTED_MODE="${EXPECTED_MODE:-}"
EXPECTED_VERSION="${EXPECTED_VERSION:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
EXAMPLES_DIR="$REPO_ROOT/docs/examples"

for required_command in curl jq
do
  if ! command -v "$required_command" >/dev/null 2>&1; then
    echo "ERROR: $required_command is required" >&2
    exit 1
  fi
done

if [[ -z "$BASE_URL" ]]; then
  echo "ERROR: OSPREY_URL is required" >&2
  exit 1
fi

if [[ -z "$ADMIN_TOKEN" ]]; then
  echo "ERROR: OSPREY_ADMIN_TOKEN is required for sandbox verification" >&2
  exit 1
fi

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

curl_json() {
  local url="$1"
  local output

  if ! output="$(curl -fsS --connect-timeout 10 --max-time 20 "$url" 2>&1)"; then
    fail "cannot reach $url: $output"
  fi

  printf '%s' "$output"
}

request_json() {
  local method="$1"
  local path="$2"
  local payload="${3:-}"
  local include_admin="${4:-true}"
  local tenant_id="${5:-$TENANT_ID}"
  local admin_scheme="${6:-bearer}"
  local output
  local curl_args=(
    -sS
    --connect-timeout 10
    --max-time 20
    -w $'\n%{http_code}'
    -X "$method"
    "$BASE_URL$path"
    -H "X-Tenant-ID: $tenant_id"
  )

  if [[ -n "$payload" ]]; then
    curl_args+=(-H "Content-Type: application/json" -d "$payload")
  fi

  if [[ "$include_admin" == "true" && -n "$ADMIN_TOKEN" ]]; then
    if [[ "$admin_scheme" == "header" ]]; then
      curl_args+=(-H "X-Osprey-Admin-Token: ${ADMIN_TOKEN}")
    else
      curl_args+=(-H "Authorization: Bearer ${ADMIN_TOKEN}")
    fi
  fi

  if ! output="$(curl "${curl_args[@]}" 2>&1)"; then
    fail "$method $path transport failed: $output"
  fi

  printf '%s' "$output"
}

body_from_response() {
  sed '$d'
}

code_from_response() {
  tail -n 1
}

expect_status() {
  local expected="$1"
  local response="$2"
  local label="$3"
  local code
  code="$(printf '%s\n' "$response" | code_from_response)"

  if [[ "$code" != "$expected" ]]; then
    echo "$response" | body_from_response >&2
    fail "$label returned HTTP $code, expected $expected"
  fi
}

echo "Verifying Osprey sandbox at $BASE_URL (tenant: $TENANT_ID)"

echo "1. Checking /health..."
health="$(curl_json "$BASE_URL/health")"
echo "$health" | jq -e '.status == "healthy" or .status == "degraded"' >/dev/null ||
  fail "/health did not return a valid status"
reported_status="$(echo "$health" | jq -r '.status')"
reported_mode="$(echo "$health" | jq -r '.mode')"
reported_version="$(echo "$health" | jq -r '.version')"
if [[ -n "$EXPECTED_STATUS" && "$reported_status" != "$EXPECTED_STATUS" ]]; then
  fail "/health reported status $reported_status, expected $EXPECTED_STATUS"
fi
if [[ -n "$EXPECTED_MODE" && "$reported_mode" != "$EXPECTED_MODE" ]]; then
  fail "/health reported mode $reported_mode, expected $EXPECTED_MODE"
fi
if [[ -n "$EXPECTED_VERSION" && "$reported_version" != "$EXPECTED_VERSION" ]]; then
  fail "/health reported version $reported_version, expected $EXPECTED_VERSION"
fi
echo "   status=$reported_status mode=$reported_mode version=$reported_version"

echo "2. Checking /ready..."
ready_response="$(request_json GET /ready)"
ready_code="$(printf '%s\n' "$ready_response" | code_from_response)"
if [[ "$ready_code" != "200" ]]; then
  echo "$ready_response" | body_from_response >&2
  fail "/ready returned HTTP $ready_code; load typologies first if this is compliance mode"
fi
echo "$ready_response" | body_from_response | jq -e '.ready == "true"' >/dev/null ||
  fail "/ready did not return ready=true"
echo "   ready=true"

rule_id="sandbox-verification-same-party"
rule_payload="$(jq -c . "$EXAMPLES_DIR/rule-same-party.json")"
typology_id="sandbox-verification-typology"
typology_payload="$(jq -c . "$EXAMPLES_DIR/typology-same-party.json")"

echo "3. Verifying configuration mutations require admin token..."
unauth_rule_response="$(request_json POST /rules "$rule_payload" false)"
expect_status 401 "$unauth_rule_response" "unauthorized POST /rules"
unauth_typology_response="$(request_json POST /typologies "$typology_payload" false)"
expect_status 401 "$unauth_typology_response" "unauthorized POST /typologies"
header_auth_response="$(request_json POST /rules/reload "" true "$TENANT_ID" header)"
expect_status 200 "$header_auth_response" "X-Osprey-Admin-Token POST /rules/reload"
echo "   unauthorized mutations blocked; X-Osprey-Admin-Token accepted"

echo "4. Creating verification rule ($rule_id)..."
rule_response="$(request_json POST /rules "$rule_payload")"
rule_code="$(printf '%s\n' "$rule_response" | code_from_response)"
if [[ ! "$rule_code" =~ ^2 ]]; then
  echo "$rule_response" | body_from_response >&2
  fail "rule creation failed with HTTP $rule_code; verify OSPREY_ADMIN_TOKEN matches the sandbox"
fi
echo "$rule_response" | body_from_response | jq -e '.rule.id == "'"$rule_id"'"' >/dev/null ||
  fail "rule creation response did not include $rule_id"
echo "   rule saved and active"

echo "5. Verifying rule appears in active engine..."
rules_response="$(request_json GET /rules)"
expect_status 200 "$rules_response" "GET /rules"
echo "$rules_response" | body_from_response | jq -e --arg id "$rule_id" '.rules[] | select(.id == $id)' >/dev/null ||
  fail "$rule_id was not found in active rules"
echo "   active rule found"

echo "6. Creating verification typology ($typology_id)..."
typology_response="$(request_json POST /typologies "$typology_payload")"
typology_code="$(printf '%s\n' "$typology_response" | code_from_response)"
if [[ ! "$typology_code" =~ ^2 ]]; then
  echo "$typology_response" | body_from_response >&2
  fail "typology creation failed with HTTP $typology_code"
fi
echo "$typology_response" | body_from_response | jq -e '.typology.id == "'"$typology_id"'"' >/dev/null ||
  fail "typology creation response did not include $typology_id"
echo "   typology saved and active"

echo "7. Verifying typology appears in active engine..."
typologies_response="$(request_json GET /typologies)"
expect_status 200 "$typologies_response" "GET /typologies"
echo "$typologies_response" | body_from_response | jq -e --arg id "$typology_id" '.typologies[] | select(.id == $id)' >/dev/null ||
  fail "$typology_id was not found in active typologies"
echo "   active typology found"

run_id="$(date +%s)-$$"
normal_tx_id="sandbox-normal-$run_id"
echo "8. Evaluating normal transaction (expect NALT)..."
normal_payload="$(jq -c --arg id "$normal_tx_id" '.id = $id' "$EXAMPLES_DIR/evaluate-normal.json")"
normal_response="$(request_json POST /evaluate "$normal_payload")"
expect_status 200 "$normal_response" "normal /evaluate"
normal_body="$(echo "$normal_response" | body_from_response)"
normal_status="$(echo "$normal_body" | jq -r '.status')"
[[ "$normal_status" == "NALT" ]] || fail "normal transaction returned $normal_status, expected NALT"
echo "   status=NALT txId=$(echo "$normal_body" | jq -r '.txId')"

alert_tx_id="sandbox-alert-$run_id"
echo "9. Evaluating same-party transaction (expect ALRT)..."
alert_payload="$(jq -c --arg id "$alert_tx_id" '.id = $id' "$EXAMPLES_DIR/evaluate-alert.json")"
alert_response="$(request_json POST /evaluate "$alert_payload")"
expect_status 200 "$alert_response" "alert /evaluate"
alert_body="$(echo "$alert_response" | body_from_response)"
alert_status="$(echo "$alert_body" | jq -r '.status')"
[[ "$alert_status" == "ALRT" ]] || fail "same-party transaction returned $alert_status, expected ALRT"
eval_id="$(echo "$alert_body" | jq -r '.evaluationId')"
tx_id="$(echo "$alert_body" | jq -r '.txId')"
[[ -n "$eval_id" && "$eval_id" != "null" ]] || fail "alert response did not include evaluationId"
[[ "$tx_id" == "$alert_tx_id" ]] || fail "alert response txId was $tx_id, expected $alert_tx_id"
echo "   status=ALRT evaluationId=$eval_id txId=$tx_id"

echo "10. Fetching persisted evaluation..."
evaluation_response="$(request_json GET "/evaluations/$eval_id")"
expect_status 200 "$evaluation_response" "GET /evaluations/$eval_id"
echo "$evaluation_response" | body_from_response | jq -e --arg id "$eval_id" '.id == $id and .status == "ALRT"' >/dev/null ||
  fail "persisted evaluation did not match alert response"
echo "   evaluation persisted"

echo "11. Fetching persisted transaction..."
transaction_response="$(request_json GET "/transactions/$alert_tx_id")"
expect_status 200 "$transaction_response" "GET /transactions/$alert_tx_id"
echo "$transaction_response" | body_from_response | jq -e --arg id "$alert_tx_id" '.id == $id and .tenantId == "'"$TENANT_ID"'"' >/dev/null ||
  fail "persisted transaction did not match submitted transaction"
echo "   transaction persisted"

echo "12. Verifying duplicate transaction protection..."
duplicate_response="$(request_json POST /evaluate "$alert_payload")"
expect_status 409 "$duplicate_response" "duplicate /evaluate"
echo "   duplicate returned HTTP 409"

isolation_tenant="${TENANT_ID}-isolation"
echo "13. Verifying tenant isolation..."
cross_tenant_response="$(request_json POST /evaluate "$alert_payload" true "$isolation_tenant")"
expect_status 200 "$cross_tenant_response" "cross-tenant /evaluate"
cross_tenant_body="$(echo "$cross_tenant_response" | body_from_response)"
cross_eval_id="$(echo "$cross_tenant_body" | jq -r '.evaluationId')"
cross_tx_id="$(echo "$cross_tenant_body" | jq -r '.txId')"
[[ "$cross_tx_id" == "$alert_tx_id" ]] || fail "cross-tenant txId was $cross_tx_id, expected $alert_tx_id"
[[ "$cross_eval_id" != "$eval_id" ]] || fail "cross-tenant evaluation reused original evaluationId"

cross_tenant_transaction_response="$(request_json GET "/transactions/$alert_tx_id" "" true "$isolation_tenant")"
expect_status 200 "$cross_tenant_transaction_response" "cross-tenant GET /transactions/$alert_tx_id"
echo "$cross_tenant_transaction_response" | body_from_response | jq -e --arg id "$alert_tx_id" --arg tenant "$isolation_tenant" '.id == $id and .tenantId == $tenant' >/dev/null ||
  fail "cross-tenant transaction did not use isolation tenant"

foreign_eval_response="$(request_json GET "/evaluations/$eval_id" "" true "$isolation_tenant")"
expect_status 404 "$foreign_eval_response" "cross-tenant GET /evaluations/$eval_id"
echo "   same transaction ID allowed across tenants and foreign evaluation hidden"

echo
echo "Sandbox verification passed."
