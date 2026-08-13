# Osprey - common dev tasks. Run `make` (or `make help`) for the list.

BINARY     := osprey
CMD        := ./cmd/osprey
GO         ?= go

VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE)

.DEFAULT_GOAL := help

## help: list targets
help:
	@grep -hE '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | \
		awk -F': ' '{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

## build: compile the binary (with version ldflags)
build:
	$(GO) build -ldflags "$(LDFLAGS)" -trimpath -o $(BINARY) $(CMD)

## run: run the engine (needs OSPREY_ADMIN_TOKEN)
run:
	$(GO) run $(CMD)

## test: run unit tests
test:
	$(GO) test -short ./...

## test-integration: boot a real server on :18080 and run the integration suite
test-integration:
	./scripts/test-integration.sh

## race: run tests with the race detector
race:
	$(GO) test -race ./...

## cover: run tests with a coverage summary
cover:
	$(GO) test -cover ./...

## vet: run go vet
vet:
	$(GO) vet ./...

## fmt: format all Go files
fmt:
	gofmt -w .

## fix: apply go fix modernizations (requires go 1.26+)
fix:
	$(GO) fix ./...

## fix-check: fail if go fix modernizations are pending (run: make fix)
fix-check:
	@$(GO) fix -diff ./... || { echo "^ modernizations pending; run 'make fix'"; exit 1; }
	@echo "fix-check: ok"

## lint: run golangci-lint (see `make tools`)
lint:
	golangci-lint run ./...
	@$(MAKE) lint-complexity

## lint-complexity: gocognit (>30) on new/changed code vs origin/main.
## Six legacy test functions exceed the threshold; refactor them over time,
## but no new or edited function may cross it. Full list: make lint-complexity WHOLE=1
lint-complexity:
	env -u GIT_DIR -u GIT_INDEX_FILE golangci-lint run ./... --enable-only=gocognit $(if $(WHOLE),,--new-from-merge-base=origin/main)

## check: gofmt drift + vet + lint + build (the CI gate)
check: vet lint build
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed on:"; gofmt -l .; exit 1; }
	@echo "check: ok"

## coverage-check: fail if unit coverage drops >0.5pt below coverage-baseline.txt
coverage-check:
	$(GO) test -short -coverprofile=coverage.out ./...
	@total=$$($(GO) tool cover -func=coverage.out | tail -1 | awk '{sub("%",""); print $$NF}'); \
	baseline=$$(cat coverage-baseline.txt); \
	echo "coverage: $$total% (baseline: $$baseline%)"; \
	awk -v t="$$total" -v b="$$baseline" 'BEGIN { exit (t < b - 0.5) ? 1 : 0 }' || \
		{ echo "FAIL: coverage $$total% is more than 0.5pt below baseline $$baseline%"; exit 1; }

## dead: report code unreachable from main (see `make tools`)
dead:
	deadcode ./cmd/osprey

## tidy: tidy go.mod / go.sum
tidy:
	$(GO) mod tidy

## tools: install golangci-lint + deadcode
tools:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
	$(GO) install golang.org/x/tools/cmd/deadcode@latest

## assure: full sandbox assurance gate (tests, race, docker, verify)
assure:
	./scripts/assure-sandbox.sh

## seed: load the FATF starter kit (needs a running server + OSPREY_ADMIN_TOKEN)
seed:
	./scripts/seed-starter-kit.sh

## ci: gofmt + go fix drift + vet + lint + race + build
ci: fix-check vet lint race build
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed on:"; gofmt -l .; exit 1; }
	@echo "ci: ok"

## clean: remove build artifacts
clean:
	rm -f $(BINARY)
	$(GO) clean

.PHONY: help build run test race cover vet fmt fix fix-check lint check coverage-check dead tidy tools assure seed ci clean hooks

hooks: ## Point git at .githooks (commit-msg shares scripts/lint-message.sh with CI)
	@chmod +x .githooks/* scripts/lint-message.sh 2>/dev/null || true
	@git config core.hooksPath .githooks
	@echo "==> git hooks active (.githooks/)"
