# Contributing to Osprey

Thank you for your help with Osprey. This guide tells you how to build, test, and submit changes.
If you contribute, you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

## Prerequisites

- **Go 1.26+**. The module targets `go 1.26`. The dev tools use `go fix` modernizers.
- `make`. This is the task runner. Run `make help` to see the full list of targets.
- Optional linters: [staticcheck](https://staticcheck.dev)
  and [deadcode](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode). To install them, run `make tools`.

## Build and test

```bash
make build        # compile the binary (version-stamped via ldflags)
make test         # go test ./...
make ci           # the full local gate. Run this before every PR.
```

`make ci` runs these checks in this sequence:

1. `go fix` drift check
2. `go vet`
3. staticcheck
4. Race tests
5. Build
6. `gofmt` check

**CI runs the same checks.** If `make ci` passes on your computer, the checks on your PR also pass.

CI and releases use more gates. These gates take more time:

```bash
make assure       # full assurance: JSON/OpenAPI/markdown/shell checks + race + integration + docker
./scripts/test-integration.sh   # HTTP integration tests against a local server
```

## Contributing rules and typologies

If you add or change detection content (not engine code), read
[docs/RULE_TYPOLOGY_AUTHORING.md](docs/RULE_TYPOLOGY_AUTHORING.md). That guide gives this information:

- The CEL variable catalog.
- The mandatory `has()` guards for optional fields.
- The test criteria that a rule must pass before promotion.

The checklist for publication is in [docs/ASSURANCE.md](docs/ASSURANCE.md).

## Shared CI standards

`pr-size.yml`, `pr-lint.yml`, and `scripts/lint-message.sh` are copies from
`josephgoksu/standards`. Do not edit those files. The policy for this repo is in `.github/standards.yml`.
That policy intentionally omits secret scanning. The repo is public and GitHub push protection is on.

## Pull requests

- **Target the `main` branch.**
- Use [Conventional Commits](https://www.conventionalcommits.org/) for messages
  (`feat:`, `fix:`, `docs:`, `chore:`, `ci:`, and other types).
- Make sure that `make ci` passes. Add tests for new behavior.
- [Documentation style](docs/STYLE.md): run `make docs-lint` before a pull request that changes docs.
- If a change has an effect on users, update [CHANGELOG.md](CHANGELOG.md) under `## [Unreleased]`.
- Keep each PR on one topic. Reviewers merge a small diff faster than a large diff.

## Reporting bugs and security issues

- For functional bugs and feature requests, open a
  [GitHub issue](https://github.com/opensource-finance/osprey/issues).
- **WARNING:** Do not report security vulnerabilities in a public issue. A public report exposes users before a fix is available.
  Follow [SECURITY.md](SECURITY.md) and report privately.

## License

If you contribute, you agree that your contributions use the
[Apache License 2.0](LICENSE). This is the same license as the project.
