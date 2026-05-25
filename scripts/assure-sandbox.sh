#!/usr/bin/env bash
# Run the sandbox assurance gate before handing Osprey to a customer.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

IMAGE_NAME="${IMAGE_NAME:-osprey:sandbox-verify}"
CONTAINER_NAME="${CONTAINER_NAME:-osprey-sandbox-verify}"
VOLUME_NAME="${VOLUME_NAME:-osprey-sandbox-verify-data}"
DOCKER_PORT="${DOCKER_PORT:-18081}"
ADMIN_TOKEN="${OSPREY_ADMIN_TOKEN:-verify-token}"
TENANT_ID="${TENANT_ID:-sandbox-assurance}"
VERSION="${VERSION:-sandbox-verify}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
SKIP_RACE="${SKIP_RACE:-false}"
SKIP_INTEGRATION="${SKIP_INTEGRATION:-false}"
SKIP_DOCKER="${SKIP_DOCKER:-false}"

cd "$REPO_ROOT"

require_command() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "ERROR: $name is required" >&2
    exit 1
  fi
}

echo "Osprey sandbox assurance"
echo "Repository: $REPO_ROOT"
echo

require_command curl
require_command go
require_command jq
require_command ruby

echo "1. Validating JSON examples..."
jq empty docs/examples/*.json

echo "2. Validating OpenAPI syntax and local references..."
ruby -e '
require "yaml"
require "json"
spec_path = "docs/api/openapi.yaml"
spec = YAML.load_file(spec_path)
errors = []

resolve_ref = lambda do |ref|
  target = spec
  ref.delete_prefix("#/").split("/").each do |part|
    part = part.gsub("~1", "/").gsub("~0", "~")
    target = target.is_a?(Hash) ? target[part] : nil
  end
  target
end

validate_schema = lambda do |schema, value, path|
  schema = resolve_ref.call(schema["$ref"]) if schema.is_a?(Hash) && schema["$ref"]
  unless schema.is_a?(Hash)
    errors << "#{path}: schema is missing"
    next
  end

  if schema["enum"] && !schema["enum"].include?(value)
    errors << "#{path}: #{value.inspect} is not one of #{schema["enum"].inspect}"
  end

  case schema["type"]
  when "object"
    unless value.is_a?(Hash)
      errors << "#{path}: expected object, got #{value.class}"
      next
    end
    Array(schema["required"]).each do |key|
      errors << "#{path}: missing required property #{key}" unless value.key?(key)
    end
    properties = schema["properties"] || {}
    if schema["additionalProperties"] == false
      extra = value.keys - properties.keys
      errors << "#{path}: unknown properties #{extra.join(", ")}" unless extra.empty?
    end
    value.each do |key, child|
      validate_schema.call(properties[key], child, "#{path}/#{key}") if properties[key]
    end
  when "array"
    unless value.is_a?(Array)
      errors << "#{path}: expected array, got #{value.class}"
      next
    end
    if schema["minItems"] && value.length < schema["minItems"]
      errors << "#{path}: expected at least #{schema["minItems"]} items"
    end
    value.each_with_index do |child, index|
      validate_schema.call(schema["items"], child, "#{path}/#{index}") if schema["items"]
    end
  when "string"
    errors << "#{path}: expected string, got #{value.class}" unless value.is_a?(String)
    if schema["format"] == "date-time" && value.is_a?(String) && !value.match?(/\A\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z\z/)
      errors << "#{path}: expected RFC3339 UTC timestamp"
    end
  when "number"
    unless value.is_a?(Numeric)
      errors << "#{path}: expected number, got #{value.class}"
      next
    end
    errors << "#{path}: must be > #{schema["exclusiveMinimum"]}" if schema.key?("exclusiveMinimum") && !(value > schema["exclusiveMinimum"])
    errors << "#{path}: must be >= #{schema["minimum"]}" if schema.key?("minimum") && value < schema["minimum"]
    errors << "#{path}: must be <= #{schema["maximum"]}" if schema.key?("maximum") && value > schema["maximum"]
  when "integer"
    errors << "#{path}: expected integer, got #{value.class}" unless value.is_a?(Integer)
  when "boolean"
    errors << "#{path}: expected boolean, got #{value.class}" unless value == true || value == false
  end
end

walk = lambda do |node, path|
  case node
  when Hash
    if node["$ref"]&.start_with?("#/")
      target = resolve_ref.call(node["$ref"])
      errors << "unresolved ref #{node["$ref"]} at #{path}" if target.nil?
    end
    if node["schema"] && node["examples"].is_a?(Hash)
      node["examples"].each do |name, example|
        next unless example["externalValue"]
        candidate = File.expand_path(example["externalValue"], File.dirname(spec_path))
        if File.file?(candidate)
          validate_schema.call(node["schema"], JSON.parse(File.read(candidate)), "#{path}/examples/#{name}")
        else
          errors << "missing externalValue #{example["externalValue"]} at #{path}/examples/#{name}"
        end
      end
    elsif node["externalValue"]
      candidate = File.expand_path(node["externalValue"], File.dirname(spec_path))
      errors << "missing externalValue #{node["externalValue"]} at #{path}" unless File.file?(candidate)
    end
    node.each { |k, v| walk.call(v, "#{path}/#{k}") }
  when Array
    node.each_with_index { |v, i| walk.call(v, "#{path}/#{i}") }
  end
end
walk.call(spec, "#")

route_source = File.read("internal/api/server.go")
route_methods = %w[Get Post Put Delete]
code_routes = route_source.scan(/(?:router|r)\.(#{route_methods.join("|")})\("([^"]+)"/).map do |method, path|
  ["#{method.upcase} #{path}"]
end.flatten.sort
openapi_routes = spec.fetch("paths").flat_map do |path, operations|
  operations.keys.grep(/\A(get|post|put|delete)\z/).map { |method| "#{method.upcase} #{path}" }
end.sort
missing_from_openapi = code_routes - openapi_routes
missing_from_code = openapi_routes - code_routes
unless missing_from_openapi.empty?
  errors << "OpenAPI missing routes from server: #{missing_from_openapi.join(", ")}"
end
unless missing_from_code.empty?
  errors << "OpenAPI documents routes not registered in server: #{missing_from_code.join(", ")}"
end

admin_routes = route_source.scan(/r\.Use\(AdminMiddleware\(cfg\.AdminToken\)\)(.*?)\n\t\t\}\)/m).flat_map do |group_body|
  group_body.first.scan(/r\.(#{route_methods.join("|")})\("([^"]+)"/).map do |method, path|
    ["#{method.upcase} #{path}"]
  end
end.flatten.sort
admin_routes.each do |route|
  method, path = route.split(" ", 2)
  operation = spec.dig("paths", path, method.downcase)
  security = Array(operation && operation["security"])
  has_admin_bearer = security.any? { |entry| entry.key?("TenantHeader") && entry.key?("AdminBearer") }
  has_admin_header = security.any? { |entry| entry.key?("TenantHeader") && entry.key?("AdminHeader") }
  unless has_admin_bearer && has_admin_header
    errors << "#{route}: OpenAPI security must include TenantHeader with AdminBearer and AdminHeader alternatives"
  end
end
non_admin_routes = openapi_routes - admin_routes - ["GET /health", "GET /ready"]
non_admin_routes.each do |route|
  method, path = route.split(" ", 2)
  operation = spec.dig("paths", path, method.downcase)
  security = Array(operation && operation["security"])
  if security.any? { |entry| entry.key?("AdminBearer") || entry.key?("AdminHeader") }
    errors << "#{route}: OpenAPI documents admin token security but server route is not admin-protected"
  end
end

abort(errors.join("\n")) unless errors.empty?
puts "   OpenAPI contract OK"
'

echo "3. Validating GitHub Actions workflow..."
ruby -e '
require "yaml"
path = ".github/workflows/sandbox-assurance.yml"
workflow = YAML.load_file(path)
errors = []
errors << "#{path}: missing name" unless workflow["name"] == "Sandbox Assurance"
errors << "#{path}: missing string on key" unless workflow.key?("on")
errors << "#{path}: YAML parser interpreted on as boolean; quote the key" if workflow.key?(true)
job = workflow.dig("jobs", "assure")
errors << "#{path}: missing assure job" unless job
if job
  errors << "#{path}: assure job must run ./scripts/assure-sandbox.sh" unless job["steps"].any? { |step| step["run"].to_s.include?("./scripts/assure-sandbox.sh") }
  install_step = job["steps"].find { |step| step["name"] == "Install assurance dependencies" }
  unless install_step && install_step["run"].to_s.include?("curl") && install_step["run"].to_s.include?("jq") && install_step["run"].to_s.include?("ruby")
    errors << "#{path}: dependency install step must include curl, jq, and ruby"
  end
end
abort(errors.join("\n")) unless errors.empty?
puts "   Workflow OK"
'

echo "4. Validating documentation links..."
ruby -e '
errors = []
markdown_files = ["README.md"] + Dir["docs/**/*.md"]
markdown_files.each do |path|
  text = File.read(path)
  text.scan(/!?\[[^\]]*\]\(([^)]+)\)/).each do |match|
    raw_target = match.first.strip
    target = raw_target.split(/\s+/, 2).first
    next if target.empty?
    next if target.start_with?("#", "http://", "https://", "mailto:")

    target = target.sub(/\A<(.+)>\z/, "\\1").split("#", 2).first
    candidate = File.expand_path(target, File.dirname(path))
    unless File.file?(candidate) || File.directory?(candidate)
      errors << "#{path}: missing link target #{raw_target}"
      next
    end

    if candidate.end_with?(".png")
      signature = File.binread(candidate, 8)
      expected = "\x89PNG\r\n\x1A\n".b
      errors << "#{path}: invalid PNG target #{raw_target}" unless signature == expected
    end
  end
end
abort(errors.join("\n")) unless errors.empty?
puts "   Markdown links OK"
'

echo "5. Validating Coolify templates..."
if [[ ! -x scripts/print-sandbox-build-args.sh ]]; then
  echo "ERROR: scripts/print-sandbox-build-args.sh must be executable" >&2
  exit 1
fi

for required_key in \
  OSPREY_MODE \
  OSPREY_TIER \
  OSPREY_DB_DRIVER \
  OSPREY_SQLITE_PATH \
  OSPREY_ADMIN_TOKEN \
  OSPREY_DEBUG
do
  if ! grep -q "^${required_key}=" docs/coolify-sandbox.env.example; then
    echo "ERROR: docs/coolify-sandbox.env.example missing $required_key" >&2
    exit 1
  fi
done

for required_key in VERSION COMMIT BUILD_DATE
do
  if ! grep -q "^${required_key}=" docs/coolify-sandbox.build-args.example; then
    echo "ERROR: docs/coolify-sandbox.build-args.example missing $required_key" >&2
    exit 1
  fi
done

build_args_output="$(VERSION="$VERSION" COMMIT="$COMMIT" BUILD_DATE="$BUILD_DATE" ./scripts/print-sandbox-build-args.sh)"
for required_key in VERSION COMMIT BUILD_DATE EXPECTED_VERSION
do
  if ! grep -q "^${required_key}=" <<<"$build_args_output"; then
    echo "ERROR: scripts/print-sandbox-build-args.sh output missing $required_key" >&2
    exit 1
  fi
done
generated_version="$(grep "^VERSION=" <<<"$build_args_output" | cut -d= -f2-)"
generated_expected_version="$(grep "^EXPECTED_VERSION=" <<<"$build_args_output" | cut -d= -f2-)"
if [[ "$generated_version" != "$generated_expected_version" ]]; then
  echo "ERROR: generated EXPECTED_VERSION does not match VERSION" >&2
  exit 1
fi
echo "   Coolify templates and build args OK"

echo "6. Checking shell script syntax..."
bash -n scripts/assure-sandbox.sh scripts/verify-sandbox.sh scripts/print-sandbox-build-args.sh scripts/seed-rules.sh scripts/seed-starter-kit.sh scripts/seed-paysim.sh scripts/test-integration.sh

echo "7. Running Go tests..."
go test ./...

echo "8. Running go vet..."
go vet ./...

if [[ "$SKIP_RACE" != "true" ]]; then
  echo "9. Running race detector..."
  go test -race ./...
else
  echo "9. Skipping race detector (SKIP_RACE=true)."
fi

echo "10. Checking diff whitespace..."
git diff --check

if [[ "$SKIP_INTEGRATION" != "true" ]]; then
  echo "11. Running HTTP integration tests..."
  ./scripts/test-integration.sh
else
  echo "11. Skipping integration tests (SKIP_INTEGRATION=true)."
fi

if [[ "$SKIP_DOCKER" != "true" ]]; then
  require_command docker

  echo "12. Building Docker image ($IMAGE_NAME)..."
  docker build \
    --build-arg VERSION="$VERSION" \
    --build-arg COMMIT="$COMMIT" \
    --build-arg BUILD_DATE="$BUILD_DATE" \
    -t "$IMAGE_NAME" .

  docker image inspect "$IMAGE_NAME" | jq -e '
    .[0].Config.User == "osprey" and
    (.[0].Config.ExposedPorts | has("8080/tcp")) and
    (.[0].Config.Volumes | has("/app/data")) and
    (.[0].Config.Healthcheck.Test | join(" ") | contains("http://localhost:8080/health"))
  ' >/dev/null || {
    echo "ERROR: Docker image metadata must include USER osprey, EXPOSE 8080, VOLUME /app/data, and /health healthcheck" >&2
    exit 1
  }
  echo "   Docker image metadata verified"

  echo "13. Running Docker sandbox verification..."
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
  docker volume rm "$VOLUME_NAME" >/dev/null 2>&1 || true
  docker volume create "$VOLUME_NAME" >/dev/null

  cleanup() {
    docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
    docker volume rm "$VOLUME_NAME" >/dev/null 2>&1 || true
  }
  trap cleanup EXIT

  docker run -d \
    --name "$CONTAINER_NAME" \
    -p "$DOCKER_PORT:8080" \
    -e OSPREY_SQLITE_PATH=/app/data/osprey.db \
    -e OSPREY_ADMIN_TOKEN="$ADMIN_TOKEN" \
    -v "$VOLUME_NAME:/app/data" \
    "$IMAGE_NAME" >/dev/null

  for _ in $(seq 1 20); do
    health_status="$(docker inspect --format='{{.State.Health.Status}}' "$CONTAINER_NAME" 2>/dev/null || true)"
    if [[ "$health_status" == "healthy" ]]; then
      break
    fi
    sleep 2
  done

  health_status="$(docker inspect --format='{{.State.Health.Status}}' "$CONTAINER_NAME")"
  if [[ "$health_status" != "healthy" ]]; then
    docker logs "$CONTAINER_NAME" >&2 || true
    echo "ERROR: Docker container health is $health_status" >&2
    exit 1
  fi

  reported_version="$(curl -fsS "http://localhost:$DOCKER_PORT/health" | jq -r '.version')"
  if [[ "$reported_version" != "$VERSION" ]]; then
    echo "ERROR: /health reported version $reported_version, expected $VERSION" >&2
    exit 1
  fi
  echo "   Docker image version verified: $reported_version"

  OSPREY_URL="http://localhost:$DOCKER_PORT" \
  OSPREY_ADMIN_TOKEN="$ADMIN_TOKEN" \
  EXPECTED_STATUS=healthy \
  EXPECTED_MODE=detection \
  EXPECTED_VERSION="$VERSION" \
  TENANT_ID="$TENANT_ID" \
    ./scripts/verify-sandbox.sh
else
  echo "12. Skipping Docker build/runtime verification (SKIP_DOCKER=true)."
fi

echo
echo "Sandbox assurance passed."
