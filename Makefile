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
	$(GO) test ./...

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

## lint: run staticcheck (see `make tools`)
lint:
	staticcheck ./...

## dead: report code unreachable from main (see `make tools`)
dead:
	deadcode ./cmd/osprey

## tidy: tidy go.mod / go.sum
tidy:
	$(GO) mod tidy

## tools: install staticcheck + deadcode
tools:
	$(GO) install honnef.co/go/tools/cmd/staticcheck@latest
	$(GO) install golang.org/x/tools/cmd/deadcode@latest

## assure: full sandbox assurance gate (tests, race, docker, verify)
assure:
	./scripts/assure-sandbox.sh

## seed: load the FATF starter kit (needs a running server + OSPREY_ADMIN_TOKEN)
seed:
	./scripts/seed-starter-kit.sh

## ci: gofmt check + vet + lint + race + build
ci: vet lint race build
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed on:"; gofmt -l .; exit 1; }
	@echo "ci: ok"

## clean: remove build artifacts
clean:
	rm -f $(BINARY)
	$(GO) clean

.PHONY: help build run test race cover vet fmt fix lint dead tidy tools assure seed ci clean
