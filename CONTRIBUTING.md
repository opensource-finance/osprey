# Contributing to Osprey

Thanks for your interest in improving Osprey. This guide covers how to build,
test, and submit changes. By participating you agree to the
[Code of Conduct](CODE_OF_CONDUCT.md).

## Prerequisites

- **Go 1.26+** (the module targets `go 1.26`; the dev tooling uses `go fix` modernizers).
- `make` (task runner — run `make help` for the full target list).
- Optional linters, installed with `make tools`: [staticcheck](https://staticcheck.dev)
  and [deadcode](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode).

## Build and test

```bash
make build        # compile the binary (version-stamped via ldflags)
make test         # go test ./...
make ci           # the full local gate — run this before every PR
```

`make ci` runs, in order: `go fix` drift check, `go vet`, staticcheck, race tests,
build, and a `gofmt` check. **CI runs the same checks**, so a green `make ci`
locally means a green PR.

Heavier gates (used in CI and before releases):

```bash
make assure       # full assurance: JSON/OpenAPI/markdown/shell checks + race + integration + docker
./scripts/test-integration.sh   # HTTP integration tests against a local server
```

## Contributing rules and typologies

If you're adding or changing detection content rather than engine code, read
[docs/RULE_TYPOLOGY_AUTHORING.md](docs/RULE_TYPOLOGY_AUTHORING.md) — it covers the
CEL variable catalog, the mandatory `has()` guards for optional fields, and the
test-before-promotion criteria. The pre-publish checklist lives in
[docs/ASSURANCE.md](docs/ASSURANCE.md).

## Pull requests

- **Target the `main` branch.**
- Use [Conventional Commits](https://www.conventionalcommits.org/) for messages
  (`feat:`, `fix:`, `docs:`, `chore:`, `ci:`, …).
- Keep `make ci` green and add tests for new behavior.
- Update [CHANGELOG.md](CHANGELOG.md) under `## [Unreleased]` for any user-facing change.
- Keep PRs focused; a small, reviewable diff lands faster than a large one.

## Reporting bugs and security issues

- Functional bugs and feature requests: open a
  [GitHub issue](https://github.com/opensource-finance/osprey/issues).
- **Security vulnerabilities: do not open a public issue.** Follow
  [SECURITY.md](SECURITY.md) and report privately.

## License

By contributing, you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE), the same license as the project.
